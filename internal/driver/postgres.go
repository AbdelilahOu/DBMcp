package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresDriver struct{}

func (d *PostgresDriver) SupportsEnums() bool { return true }

func (d *PostgresDriver) SupportsSequences() bool { return true }

func (d *PostgresDriver) SupportsMaterializedViews() bool { return true }

func (d *PostgresDriver) SupportsFunctions() bool { return true }

func (d *PostgresDriver) SupportsShowCommands() bool { return true }

func (d *PostgresDriver) defaultSchema(schema string) string {
	if schema == "" {
		return "public"
	}
	return schema
}

func (d *PostgresDriver) ListSchemas(ctx context.Context, conn *sql.DB) ([]SchemaRow, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast') ORDER BY schema_name`)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var schemas []SchemaRow
	for rows.Next() {
		var s SchemaRow
		if err := rows.Scan(&s.Name); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
}

func (d *PostgresDriver) GetDbInfo(ctx context.Context, conn *sql.DB) (DbInfo, error) {
	var info DbInfo

	if err := conn.QueryRowContext(ctx, "SELECT current_database()").Scan(&info.DatabaseName); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get database name: %v", err)
	}

	var version string
	if err := conn.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get version: %v", err)
	}
	if strings.Contains(version, "PostgreSQL") {
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			version = "PostgreSQL " + parts[1]
		}
	}
	info.Version = version

	schemas, err := queryStringSlice(ctx, conn, "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')")
	if err != nil {
		return DbInfo{}, fmt.Errorf("failed to get schemas: %v", err)
	}
	info.Schemas = schemas

	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast')").Scan(&info.TableCount); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get table count: %v", err)
	}

	return info, nil
}

func (d *PostgresDriver) ListTables(ctx context.Context, conn *sql.DB, schema string) ([]TableRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name`
		args = append(args, schema)
	} else {
		query = `SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'pg_catalog') ORDER BY table_name`
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var tables []TableRow
	for rows.Next() {
		var t TableRow
		var tableType string
		if err := rows.Scan(&t.Name, &t.Schema, &tableType); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		t.Type = normalizeTableType(tableType)
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (d *PostgresDriver) DescribeTable(ctx context.Context, conn *sql.DB, table, schema string) (DescribeTableResult, error) {
	schema = d.defaultSchema(schema)
	columns, err := d.getColumns(ctx, conn, table, schema)
	if err != nil {
		return DescribeTableResult{}, err
	}
	indexes, err := d.getIndexes(ctx, conn, table, schema)
	if err != nil {
		return DescribeTableResult{}, err
	}
	return DescribeTableResult{Columns: columns, Indexes: indexes}, nil
}

func (d *PostgresDriver) getColumns(ctx context.Context, conn *sql.DB, table, schema string) ([]ColumnRow, error) {
	query := `
		SELECT
			c.column_name, c.data_type,
			CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END,
			COALESCE(c.column_default, ''), c.character_maximum_length,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_name = $1 AND tc.table_schema = $2
		) pk ON c.column_name = pk.column_name
		WHERE c.table_name = $1 AND c.table_schema = $2
		ORDER BY c.ordinal_position`

	rows, err := conn.QueryContext(ctx, query, table, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []ColumnRow
	for rows.Next() {
		var col ColumnRow
		var charMaxLen sql.NullInt64
		if err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.DefaultValue, &charMaxLen, &col.IsPrimaryKey); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		if charMaxLen.Valid {
			l := int(charMaxLen.Int64)
			col.CharMaxLength = &l
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (d *PostgresDriver) getIndexes(ctx context.Context, conn *sql.DB, table, schema string) ([]IndexRow, error) {
	query := `
		SELECT i.relname, array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)), ix.indisunique
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = $1 AND n.nspname = $2
		GROUP BY i.relname, ix.indisunique ORDER BY i.relname`

	rows, err := conn.QueryContext(ctx, query, table, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	var indexes []IndexRow
	for rows.Next() {
		var idx IndexRow
		var colsStr string
		if err := rows.Scan(&idx.Name, &colsStr, &idx.IsUnique); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		idx.Columns = parsePostgresArray(colsStr)
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

func (d *PostgresDriver) AnalyzeTable(ctx context.Context, conn *sql.DB, table, schema string) (TableStats, error) {
	schema = d.defaultSchema(schema)
	stats := TableStats{TableName: table, ColumnStats: make(map[string]string)}

	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+schema+`"."`+table+`"`).Scan(&stats.RowCount); err != nil {
		return TableStats{}, fmt.Errorf("failed to get row count: %v", err)
	}

	sizeQuery := `
		SELECT
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)),
			pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)),
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename))
		FROM pg_tables WHERE schemaname = $1 AND tablename = $2`
	if err := conn.QueryRowContext(ctx, sizeQuery, schema, table).Scan(&stats.TotalSize, &stats.TableSize, &stats.IndexSize); err != nil {
		stats.TotalSize, stats.TableSize, stats.IndexSize = "N/A", "N/A", "N/A"
	}

	var lastAnalyze, lastAutoAnalyze sql.NullTime
	if err := conn.QueryRowContext(ctx, "SELECT last_analyze, last_autoanalyze FROM pg_stat_user_tables WHERE schemaname = $1 AND relname = $2", schema, table).Scan(&lastAnalyze, &lastAutoAnalyze); err == nil {
		if lastAnalyze.Valid {
			stats.LastAnalyzed = lastAnalyze.Time.Format("2006-01-02 15:04:05")
		} else if lastAutoAnalyze.Valid {
			stats.LastAnalyzed = lastAutoAnalyze.Time.Format("2006-01-02 15:04:05") + " (auto)"
		} else {
			stats.LastAnalyzed = "Never"
		}
	} else {
		stats.LastAnalyzed = "N/A"
	}

	colRows, err := conn.QueryContext(ctx,
		`SELECT attname, CASE WHEN attnotnull THEN 'Not Null' ELSE 'Nullable' END
		 FROM pg_attribute a JOIN pg_class t ON a.attrelid = t.oid JOIN pg_namespace n ON t.relnamespace = n.oid
		 WHERE n.nspname = $1 AND t.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped LIMIT 5`,
		schema, table)
	if err == nil {
		defer colRows.Close()
		for colRows.Next() {
			var colName, nullability string
			if err := colRows.Scan(&colName, &nullability); err == nil {
				stats.ColumnStats[colName] = nullability
			}
		}
	}

	return stats, nil
}

func (d *PostgresDriver) ListViews(ctx context.Context, conn *sql.DB, schema string) ([]ViewRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT table_name, table_schema FROM information_schema.views WHERE table_schema = $1 ORDER BY table_name`
		args = append(args, schema)
	} else {
		query = `SELECT table_name, table_schema FROM information_schema.views WHERE table_schema NOT IN ('information_schema', 'pg_catalog') ORDER BY table_name`
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var views []ViewRow
	for rows.Next() {
		var v ViewRow
		if err := rows.Scan(&v.Name, &v.Schema); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (d *PostgresDriver) GetViewDefinition(ctx context.Context, conn *sql.DB, view, schema string) (ViewDefinition, error) {
	schema = d.defaultSchema(schema)
	var def ViewDefinition
	def.ViewName = view
	def.Schema = schema
	err := conn.QueryRowContext(ctx, `SELECT view_definition FROM information_schema.views WHERE table_name = $1 AND table_schema = $2`, view, schema).Scan(&def.Definition)
	if err == sql.ErrNoRows {
		return ViewDefinition{}, fmt.Errorf("view '%s' not found in schema '%s'", view, schema)
	}
	if err != nil {
		return ViewDefinition{}, fmt.Errorf("query error: %v", err)
	}
	return def, nil
}

func (d *PostgresDriver) ListMaterializedViews(ctx context.Context, conn *sql.DB, schema string) ([]MaterializedViewRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT matviewname, schemaname FROM pg_matviews WHERE schemaname = $1 ORDER BY matviewname`
		args = append(args, schema)
	} else {
		query = `SELECT matviewname, schemaname FROM pg_matviews WHERE schemaname NOT IN ('information_schema', 'pg_catalog') ORDER BY matviewname`
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var views []MaterializedViewRow
	for rows.Next() {
		var v MaterializedViewRow
		if err := rows.Scan(&v.Name, &v.Schema); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (d *PostgresDriver) ListForeignKeys(ctx context.Context, conn *sql.DB, table, schema string) ([]ForeignKeyRow, error) {
	schema = d.defaultSchema(schema)
	query := `
		SELECT tc.constraint_name, tc.table_schema, tc.table_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position),
			ccu.table_schema, ccu.table_name,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1`
	args := []interface{}{schema}
	if table != "" {
		query += " AND tc.table_name = $2"
		args = append(args, table)
	}
	query += ` GROUP BY tc.constraint_name, tc.table_schema, tc.table_name, ccu.table_schema, ccu.table_name, rc.delete_rule, rc.update_rule ORDER BY tc.table_name, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var fks []ForeignKeyRow
	for rows.Next() {
		var fk ForeignKeyRow
		var cols, refCols string
		if err := rows.Scan(&fk.ConstraintName, &fk.TableSchema, &fk.TableName, &cols, &fk.ReferencedSchema, &fk.ReferencedTable, &refCols, &fk.OnDelete, &fk.OnUpdate); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		fk.Columns = parsePostgresArray(cols)
		fk.ReferencedColumns = parsePostgresArray(refCols)
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (d *PostgresDriver) GetTableRelationships(ctx context.Context, conn *sql.DB, table, schema string) ([]RelationshipRow, error) {
	schema = d.defaultSchema(schema)
	outQ := `
		SELECT 'outgoing', tc.constraint_name, ccu.table_schema, ccu.table_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position),
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1 AND tc.table_schema = $2
		GROUP BY tc.constraint_name, ccu.table_schema, ccu.table_name, rc.delete_rule, rc.update_rule`

	inQ := `
		SELECT 'incoming', tc.constraint_name, tc.table_schema, tc.table_name,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position),
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc ON tc.constraint_name = rc.constraint_name AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = $1 AND ccu.table_schema = $2
		GROUP BY tc.constraint_name, tc.table_schema, tc.table_name, rc.delete_rule, rc.update_rule`

	var rels []RelationshipRow
	for _, q := range []string{outQ, inQ} {
		rows, err := conn.QueryContext(ctx, q, table, schema)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		for rows.Next() {
			var r RelationshipRow
			var cols, relCols string
			if err := rows.Scan(&r.Type, &r.ConstraintName, &r.RelatedSchema, &r.RelatedTable, &cols, &relCols, &r.OnDelete, &r.OnUpdate); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan error: %v", err)
			}
			r.Columns = parsePostgresArray(cols)
			r.RelatedColumns = parsePostgresArray(relCols)
			rels = append(rels, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return rels, nil
}

func (d *PostgresDriver) ListTriggers(ctx context.Context, conn *sql.DB, table, schema string) ([]TriggerRow, error) {
	schema = d.defaultSchema(schema)
	query := `
		SELECT trigger_name, trigger_schema, event_object_table, event_manipulation, action_timing,
			CASE WHEN action_orientation = 'ROW' THEN true ELSE false END
		FROM information_schema.triggers WHERE trigger_schema = $1`
	args := []interface{}{schema}
	if table != "" {
		query += " AND event_object_table = $2"
		args = append(args, table)
	}
	query += " ORDER BY event_object_table, trigger_name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var triggers []TriggerRow
	for rows.Next() {
		var t TriggerRow
		if err := rows.Scan(&t.Name, &t.Schema, &t.TableName, &t.Event, &t.Timing, &t.ForEachRow); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

func (d *PostgresDriver) GetTriggerDefinition(ctx context.Context, conn *sql.DB, trigger, schema string) (TriggerDefinition, error) {
	schema = d.defaultSchema(schema)
	query := `
		SELECT t.trigger_name, t.trigger_schema, t.event_object_table, t.event_manipulation, t.action_timing,
			pg_get_triggerdef(pg_trigger.oid)
		FROM information_schema.triggers t
		JOIN pg_trigger ON pg_trigger.tgname = t.trigger_name
		JOIN pg_class ON pg_class.oid = pg_trigger.tgrelid
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE t.trigger_name = $1 AND t.trigger_schema = $2 AND pg_namespace.nspname = $2`

	var def TriggerDefinition
	err := conn.QueryRowContext(ctx, query, trigger, schema).Scan(&def.TriggerName, &def.Schema, &def.TableName, &def.Event, &def.Timing, &def.Definition)
	if err == sql.ErrNoRows {
		return TriggerDefinition{}, fmt.Errorf("trigger '%s' not found in schema '%s'", trigger, schema)
	}
	if err != nil {
		return TriggerDefinition{}, fmt.Errorf("query error: %v", err)
	}
	return def, nil
}

func (d *PostgresDriver) ListFunctions(ctx context.Context, conn *sql.DB, schema string, filterBySchema bool) ([]FunctionRow, error) {
	query := `
		SELECT p.proname, n.nspname, pg_catalog.pg_get_function_result(p.oid), l.lanname
		FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid JOIN pg_language l ON p.prolang = l.oid
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')`
	var args []interface{}
	if filterBySchema {
		query += " AND n.nspname = $1"
		args = append(args, schema)
	}
	query += " ORDER BY n.nspname, p.proname"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var functions []FunctionRow
	for rows.Next() {
		var f FunctionRow
		if err := rows.Scan(&f.Name, &f.Schema, &f.ReturnType, &f.Language); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		functions = append(functions, f)
	}
	return functions, rows.Err()
}

func (d *PostgresDriver) GetFunctionDefinition(ctx context.Context, conn *sql.DB, fn, schema string) (FunctionDefinition, error) {
	query := `
		SELECT p.proname, n.nspname, pg_catalog.pg_get_function_result(p.oid), l.lanname,
			pg_catalog.pg_get_functiondef(p.oid), pg_catalog.pg_get_function_arguments(p.oid)
		FROM pg_proc p JOIN pg_namespace n ON p.pronamespace = n.oid JOIN pg_language l ON p.prolang = l.oid
		WHERE p.proname = $1 AND n.nspname = $2`

	var def FunctionDefinition
	err := conn.QueryRowContext(ctx, query, fn, schema).Scan(&def.FunctionName, &def.Schema, &def.ReturnType, &def.Language, &def.Definition, &def.Arguments)
	if err == sql.ErrNoRows {
		return FunctionDefinition{}, fmt.Errorf("function '%s' not found in schema '%s'", fn, schema)
	}
	if err != nil {
		return FunctionDefinition{}, fmt.Errorf("query error: %v", err)
	}
	return def, nil
}

func (d *PostgresDriver) ListConstraints(ctx context.Context, conn *sql.DB, table, schema string) ([]ConstraintRow, error) {
	schema = d.defaultSchema(schema)
	query := `
		SELECT tc.constraint_name, tc.constraint_type, tc.table_schema, tc.table_name,
			array_agg(DISTINCT kcu.column_name ORDER BY kcu.column_name),
			COALESCE(cc.check_clause, ''), COALESCE(ccu.table_schema, ''), COALESCE(ccu.table_name, '')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.check_constraints cc ON tc.constraint_name = cc.constraint_name AND tc.constraint_schema = cc.constraint_schema
		LEFT JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.table_schema = $1`
	args := []interface{}{schema}
	if table != "" {
		query += " AND tc.table_name = $2"
		args = append(args, table)
	}
	query += ` GROUP BY tc.constraint_name, tc.constraint_type, tc.table_schema, tc.table_name, cc.check_clause, ccu.table_schema, ccu.table_name ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var constraints []ConstraintRow
	for rows.Next() {
		var c ConstraintRow
		var colsStr string
		if err := rows.Scan(&c.ConstraintName, &c.ConstraintType, &c.TableSchema, &c.TableName, &colsStr, &c.CheckClause, &c.ReferencedSchema, &c.ReferencedTable); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		c.Columns = parsePostgresArray(colsStr)
		constraints = append(constraints, c)
	}
	return constraints, rows.Err()
}

func (d *PostgresDriver) FindColumns(ctx context.Context, conn *sql.DB, column, table, schema string, exactMatch bool) ([]ColumnMatch, error) {
	query := `SELECT table_name, table_schema, column_name, data_type, CASE WHEN is_nullable = 'YES' THEN true ELSE false END, ordinal_position FROM information_schema.columns WHERE table_schema NOT IN ('information_schema', 'pg_catalog')`
	var args []interface{}
	argN := 0
	if schema != "" {
		argN++
		query += fmt.Sprintf(" AND table_schema = $%d", argN)
		args = append(args, schema)
	}
	if table != "" {
		argN++
		query += fmt.Sprintf(" AND table_name = $%d", argN)
		args = append(args, table)
	}
	argN++
	if exactMatch {
		query += fmt.Sprintf(" AND column_name = $%d", argN)
		args = append(args, column)
	} else {
		query += fmt.Sprintf(" AND column_name ILIKE $%d", argN)
		args = append(args, "%"+column+"%")
	}
	query += " ORDER BY table_schema, table_name, ordinal_position"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var cols []ColumnMatch
	for rows.Next() {
		var c ColumnMatch
		if err := rows.Scan(&c.TableName, &c.TableSchema, &c.ColumnName, &c.DataType, &c.IsNullable, &c.Position); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (d *PostgresDriver) ListSequences(ctx context.Context, conn *sql.DB, schema string) ([]SequenceRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT sequencename, schemaname FROM pg_sequences WHERE schemaname = $1 ORDER BY sequencename`
		args = append(args, schema)
	} else {
		query = `SELECT sequencename, schemaname FROM pg_sequences WHERE schemaname NOT IN ('information_schema', 'pg_catalog') ORDER BY sequencename`
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var seqs []SequenceRow
	for rows.Next() {
		var s SequenceRow
		if err := rows.Scan(&s.Name, &s.Schema); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		seqs = append(seqs, s)
	}
	return seqs, rows.Err()
}

func (d *PostgresDriver) GetSequenceInfo(ctx context.Context, conn *sql.DB, sequence, schema string) (SequenceDetails, error) {
	schema = d.defaultSchema(schema)
	det := SequenceDetails{SequenceName: sequence, Schema: schema}
	var minVal, maxVal sql.NullInt64
	err := conn.QueryRowContext(ctx,
		`SELECT last_value, start_value, increment_by, min_value, max_value, cache_size, cycle FROM pg_sequences WHERE sequencename = $1 AND schemaname = $2`,
		sequence, schema).Scan(&det.LastValue, &det.StartValue, &det.IncrementBy, &minVal, &maxVal, &det.CacheSize, &det.Cycle)
	if err == sql.ErrNoRows {
		return SequenceDetails{}, fmt.Errorf("sequence '%s' not found in schema '%s'", sequence, schema)
	}
	if err != nil {
		return SequenceDetails{}, fmt.Errorf("query error: %v", err)
	}
	if minVal.Valid {
		v := minVal.Int64
		det.MinValue = &v
	}
	if maxVal.Valid {
		v := maxVal.Int64
		det.MaxValue = &v
	}
	return det, nil
}

func (d *PostgresDriver) ListEnums(ctx context.Context, conn *sql.DB, schema string) ([]EnumRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT t.typname, n.nspname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typtype = 'e' AND n.nspname = $1 ORDER BY n.nspname, t.typname`
		args = append(args, schema)
	} else {
		query = `SELECT t.typname, n.nspname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typtype = 'e' AND n.nspname NOT IN ('information_schema', 'pg_catalog') ORDER BY n.nspname, t.typname`
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var enums []EnumRow
	for rows.Next() {
		var e EnumRow
		if err := rows.Scan(&e.Name, &e.Schema); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		enums = append(enums, e)
	}
	return enums, rows.Err()
}

func (d *PostgresDriver) GetEnumValues(ctx context.Context, conn *sql.DB, enum, schema string) ([]string, error) {
	schema = d.defaultSchema(schema)
	rows, err := conn.QueryContext(ctx,
		`SELECT e.enumlabel FROM pg_type t JOIN pg_enum e ON t.oid = e.enumtypid JOIN pg_namespace n ON t.typnamespace = n.oid WHERE t.typname = $1 AND n.nspname = $2 ORDER BY e.enumsortorder`,
		enum, schema)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func queryStringSlice(ctx context.Context, conn *sql.DB, query string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
