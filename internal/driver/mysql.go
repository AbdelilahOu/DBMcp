package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type MysqlDriver struct{}

func (d *MysqlDriver) SupportsEnums() bool             { return false }
func (d *MysqlDriver) SupportsSequences() bool         { return false }
func (d *MysqlDriver) SupportsMaterializedViews() bool { return false }
func (d *MysqlDriver) SupportsFunctions() bool         { return true }
func (d *MysqlDriver) SupportsShowCommands() bool      { return true }

func (d *MysqlDriver) ListSchemas(ctx context.Context, conn *sql.DB) ([]SchemaRow, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys') ORDER BY schema_name`)
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

func (d *MysqlDriver) GetDbInfo(ctx context.Context, conn *sql.DB) (DbInfo, error) {
	var info DbInfo

	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&info.DatabaseName); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get database name: %v", err)
	}

	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get version: %v", err)
	}
	info.Version = "MySQL " + version

	schemas, err := queryMySQLStringSlice(ctx, conn, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')")
	if err != nil {
		return DbInfo{}, fmt.Errorf("failed to get schemas: %v", err)
	}
	info.Schemas = schemas

	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')").Scan(&info.TableCount); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get table count: %v", err)
	}

	return info, nil
}

func (d *MysqlDriver) ListTables(ctx context.Context, conn *sql.DB, schema string) ([]TableRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name`
		args = append(args, schema)
	} else {
		query = `SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY table_name`
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

func (d *MysqlDriver) DescribeTable(ctx context.Context, conn *sql.DB, table, schema string) (DescribeTableResult, error) {
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

func (d *MysqlDriver) getColumns(ctx context.Context, conn *sql.DB, table, schema string) ([]ColumnRow, error) {
	query := `
		SELECT COLUMN_NAME, DATA_TYPE,
			CASE WHEN IS_NULLABLE = 'YES' THEN true ELSE false END,
			COALESCE(COLUMN_DEFAULT, ''), CHARACTER_MAXIMUM_LENGTH,
			CASE WHEN COLUMN_KEY = 'PRI' THEN true ELSE false END
		FROM information_schema.columns
		WHERE table_name = ? AND table_schema = ?
		ORDER BY ordinal_position`

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

func (d *MysqlDriver) getIndexes(ctx context.Context, conn *sql.DB, table, schema string) ([]IndexRow, error) {
	query := `
		SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX), CASE WHEN NON_UNIQUE = 0 THEN true ELSE false END
		FROM information_schema.statistics
		WHERE table_name = ? AND table_schema = ?
		GROUP BY index_name, non_unique ORDER BY index_name`

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
		idx.Columns = parseMySQLArray(colsStr)
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

func (d *MysqlDriver) AnalyzeTable(ctx context.Context, conn *sql.DB, table, schema string) (TableStats, error) {
	stats := TableStats{TableName: table, ColumnStats: make(map[string]string)}

	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+schema+"`.`"+table+"`").Scan(&stats.RowCount); err != nil {
		return TableStats{}, fmt.Errorf("failed to get row count: %v", err)
	}

	sizeQuery := `
		SELECT ROUND(((data_length + index_length) / 1024 / 1024), 2),
			ROUND((data_length / 1024 / 1024), 2), ROUND((index_length / 1024 / 1024), 2)
		FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`
	var totalMB, tableMB, indexMB sql.NullFloat64
	if err := conn.QueryRowContext(ctx, sizeQuery, schema, table).Scan(&totalMB, &tableMB, &indexMB); err != nil {
		return TableStats{}, fmt.Errorf("failed to get size information: %v", err)
	}
	if totalMB.Valid {
		stats.TotalSize = fmt.Sprintf("%.2f MB", totalMB.Float64)
	} else {
		stats.TotalSize = "N/A"
	}
	if tableMB.Valid {
		stats.TableSize = fmt.Sprintf("%.2f MB", tableMB.Float64)
	} else {
		stats.TableSize = "N/A"
	}
	if indexMB.Valid {
		stats.IndexSize = fmt.Sprintf("%.2f MB", indexMB.Float64)
	} else {
		stats.IndexSize = "N/A"
	}
	stats.LastAnalyzed = "N/A (MySQL)"

	colRows, err := conn.QueryContext(ctx,
		`SELECT COLUMN_NAME, CASE WHEN IS_NULLABLE = 'YES' THEN 'Nullable' ELSE 'Not Null' END FROM information_schema.columns WHERE table_schema = ? AND table_name = ? LIMIT 5`,
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

func (d *MysqlDriver) ListViews(ctx context.Context, conn *sql.DB, schema string) ([]ViewRow, error) {
	var query string
	var args []interface{}
	if schema != "" {
		query = `SELECT table_name, table_schema FROM information_schema.views WHERE table_schema = ? ORDER BY table_name`
		args = append(args, schema)
	} else {
		query = `SELECT table_name, table_schema FROM information_schema.views WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY table_name`
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

func (d *MysqlDriver) GetViewDefinition(ctx context.Context, conn *sql.DB, view, schema string) (ViewDefinition, error) {
	var def ViewDefinition
	def.ViewName = view
	def.Schema = schema
	err := conn.QueryRowContext(ctx, `SELECT view_definition FROM information_schema.views WHERE table_name = ? AND table_schema = ?`, view, schema).Scan(&def.Definition)
	if err == sql.ErrNoRows {
		return ViewDefinition{}, fmt.Errorf("view '%s' not found in schema '%s'", view, schema)
	}
	if err != nil {
		return ViewDefinition{}, fmt.Errorf("query error: %v", err)
	}
	return def, nil
}

func (d *MysqlDriver) ListMaterializedViews(ctx context.Context, conn *sql.DB, schema string) ([]MaterializedViewRow, error) {
	return nil, fmt.Errorf("MySQL does not support materialized views")
}

func (d *MysqlDriver) ListForeignKeys(ctx context.Context, conn *sql.DB, table, schema string) ([]ForeignKeyRow, error) {
	query := `
		SELECT kcu.constraint_name, kcu.table_schema, kcu.table_name,
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position),
			kcu.referenced_table_schema, kcu.referenced_table_name,
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc ON kcu.constraint_name = rc.constraint_name AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name IS NOT NULL AND kcu.table_schema = ?`
	args := []interface{}{schema}
	if table != "" {
		query += " AND kcu.table_name = ?"
		args = append(args, table)
	}
	query += ` GROUP BY kcu.constraint_name, kcu.table_schema, kcu.table_name, kcu.referenced_table_schema, kcu.referenced_table_name, rc.delete_rule, rc.update_rule ORDER BY kcu.table_name, kcu.constraint_name`

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
		fk.Columns = parseMySQLArray(cols)
		fk.ReferencedColumns = parseMySQLArray(refCols)
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (d *MysqlDriver) GetTableRelationships(ctx context.Context, conn *sql.DB, table, schema string) ([]RelationshipRow, error) {
	outQ := `
		SELECT 'outgoing', kcu.constraint_name, kcu.referenced_table_schema, kcu.referenced_table_name,
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position),
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc ON kcu.constraint_name = rc.constraint_name AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name IS NOT NULL AND kcu.table_name = ? AND kcu.table_schema = ?
		GROUP BY kcu.constraint_name, kcu.referenced_table_schema, kcu.referenced_table_name, rc.delete_rule, rc.update_rule`

	inQ := `
		SELECT 'incoming', kcu.constraint_name, kcu.table_schema, kcu.table_name,
			GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position),
			GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position),
			rc.delete_rule, rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc ON kcu.constraint_name = rc.constraint_name AND kcu.table_schema = rc.constraint_schema
		WHERE kcu.referenced_table_name = ? AND kcu.referenced_table_schema = ?
		GROUP BY kcu.constraint_name, kcu.table_schema, kcu.table_name, rc.delete_rule, rc.update_rule`

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
			r.Columns = parseMySQLArray(cols)
			r.RelatedColumns = parseMySQLArray(relCols)
			rels = append(rels, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return rels, nil
}

func (d *MysqlDriver) ListTriggers(ctx context.Context, conn *sql.DB, table, schema string) ([]TriggerRow, error) {
	query := `
		SELECT trigger_name, trigger_schema, event_object_table, event_manipulation, action_timing, true
		FROM information_schema.triggers WHERE trigger_schema = ?`
	args := []interface{}{schema}
	if table != "" {
		query += " AND event_object_table = ?"
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

func (d *MysqlDriver) GetTriggerDefinition(ctx context.Context, conn *sql.DB, trigger, schema string) (TriggerDefinition, error) {
	query := `
		SELECT trigger_name, trigger_schema, event_object_table, event_manipulation, action_timing, action_statement
		FROM information_schema.triggers WHERE trigger_name = ? AND trigger_schema = ?`

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

func (d *MysqlDriver) ListFunctions(ctx context.Context, conn *sql.DB, schema string, filterBySchema bool) ([]FunctionRow, error) {
	query := `
		SELECT routine_name, routine_schema, COALESCE(dtd_identifier, 'void'), 'SQL'
		FROM information_schema.routines
		WHERE routine_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')`
	var args []interface{}
	if filterBySchema {
		query += " AND routine_schema = ?"
		args = append(args, schema)
	}
	query += " ORDER BY routine_schema, routine_name"

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

func (d *MysqlDriver) GetFunctionDefinition(ctx context.Context, conn *sql.DB, fn, schema string) (FunctionDefinition, error) {
	query := `
		SELECT routine_name, routine_schema, COALESCE(dtd_identifier, 'void'), 'SQL', routine_definition, COALESCE(parameter_style, '')
		FROM information_schema.routines WHERE routine_name = ? AND routine_schema = ?`

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

func (d *MysqlDriver) ListConstraints(ctx context.Context, conn *sql.DB, table, schema string) ([]ConstraintRow, error) {
	query := `
		SELECT tc.constraint_name, tc.constraint_type, tc.table_schema, tc.table_name,
			GROUP_CONCAT(DISTINCT kcu.column_name ORDER BY kcu.ordinal_position),
			COALESCE(cc.check_clause, ''),
			COALESCE(kcu.referenced_table_schema, ''), COALESCE(kcu.referenced_table_name, '')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema AND tc.table_name = kcu.table_name
		LEFT JOIN information_schema.check_constraints cc ON tc.constraint_name = cc.constraint_name AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.table_schema = ?`
	args := []interface{}{schema}
	if table != "" {
		query += " AND tc.table_name = ?"
		args = append(args, table)
	}
	query += ` GROUP BY tc.constraint_name, tc.constraint_type, tc.table_schema, tc.table_name, cc.check_clause, kcu.referenced_table_schema, kcu.referenced_table_name ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var constraints []ConstraintRow
	for rows.Next() {
		var c ConstraintRow
		var colsStr sql.NullString
		if err := rows.Scan(&c.ConstraintName, &c.ConstraintType, &c.TableSchema, &c.TableName, &colsStr, &c.CheckClause, &c.ReferencedSchema, &c.ReferencedTable); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		if colsStr.Valid {
			c.Columns = parseMySQLArray(colsStr.String)
		}
		constraints = append(constraints, c)
	}
	return constraints, rows.Err()
}

func (d *MysqlDriver) FindColumns(ctx context.Context, conn *sql.DB, column, table, schema string, exactMatch bool) ([]ColumnMatch, error) {
	query := `SELECT table_name, table_schema, column_name, data_type, CASE WHEN is_nullable = 'YES' THEN true ELSE false END, ordinal_position FROM information_schema.columns WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')`
	var args []interface{}
	if schema != "" {
		query += " AND table_schema = ?"
		args = append(args, schema)
	}
	if table != "" {
		query += " AND table_name = ?"
		args = append(args, table)
	}
	if exactMatch {
		query += " AND column_name = ?"
		args = append(args, column)
	} else {
		query += " AND column_name LIKE ?"
		args = append(args, "%"+column+"%")
	}
	query += " ORDER BY table_schema, table_name, ordinal_position"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	search := strings.ToLower(column)
	var cols []ColumnMatch
	for rows.Next() {
		var c ColumnMatch
		if err := rows.Scan(&c.TableName, &c.TableSchema, &c.ColumnName, &c.DataType, &c.IsNullable, &c.Position); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		if !exactMatch && !strings.Contains(strings.ToLower(c.ColumnName), search) {
			continue
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (d *MysqlDriver) ListSequences(ctx context.Context, conn *sql.DB, schema string) ([]SequenceRow, error) {
	return nil, fmt.Errorf("MySQL does not support sequences. Use AUTO_INCREMENT columns instead")
}

func (d *MysqlDriver) GetSequenceInfo(ctx context.Context, conn *sql.DB, sequence, schema string) (SequenceDetails, error) {
	return SequenceDetails{}, fmt.Errorf("MySQL does not support sequences. Use AUTO_INCREMENT columns instead")
}

func (d *MysqlDriver) ListEnums(ctx context.Context, conn *sql.DB, schema string) ([]EnumRow, error) {
	return nil, fmt.Errorf("MySQL does not support standalone ENUM types. Use describe_table to see column-level ENUM definitions")
}

func (d *MysqlDriver) GetEnumValues(ctx context.Context, conn *sql.DB, enum, schema string) ([]string, error) {
	return nil, fmt.Errorf("MySQL does not support standalone ENUM types. Use describe_table to see column-level ENUM definitions")
}

func queryMySQLStringSlice(ctx context.Context, conn *sql.DB, query string) ([]string, error) {
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
