package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SqliteDriver struct{}

func (d *SqliteDriver) SupportsEnums() bool { return false }

func (d *SqliteDriver) SupportsSequences() bool { return false }

func (d *SqliteDriver) SupportsMaterializedViews() bool { return false }

func (d *SqliteDriver) SupportsFunctions() bool { return false }

func (d *SqliteDriver) SupportsShowCommands() bool { return false }

func (d *SqliteDriver) ListSchemas(ctx context.Context, conn *sql.DB) ([]SchemaRow, error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var schemas []SchemaRow
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		schemas = append(schemas, SchemaRow{Name: name})
	}
	return schemas, rows.Err()
}

func (d *SqliteDriver) GetDbInfo(ctx context.Context, conn *sql.DB) (DbInfo, error) {
	var info DbInfo

	var sqliteVer string
	if err := conn.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&sqliteVer); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get version: %v", err)
	}
	info.Version = "SQLite " + sqliteVer

	rows, err := conn.QueryContext(ctx, "PRAGMA database_list")
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var seq int
			var name, dbFile string
			_ = rows.Scan(&seq, &name, &dbFile)
			if dbFile == "" {
				info.DatabaseName = name
			} else {
				info.DatabaseName = dbFile
			}
		}
	}
	if info.DatabaseName == "" {
		info.DatabaseName = "sqlite"
	}

	info.Schemas = []string{"main"}

	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&info.TableCount); err != nil {
		return DbInfo{}, fmt.Errorf("failed to get table count: %v", err)
	}

	return info, nil
}

func (d *SqliteDriver) ListTables(ctx context.Context, conn *sql.DB, schema string) ([]TableRow, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name, 'main', 'table' FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var tables []TableRow
	for rows.Next() {
		var t TableRow
		if err := rows.Scan(&t.Name, &t.Schema, &t.Type); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (d *SqliteDriver) DescribeTable(ctx context.Context, conn *sql.DB, table, schema string) (DescribeTableResult, error) {
	columns, err := d.getColumns(ctx, conn, table)
	if err != nil {
		return DescribeTableResult{}, err
	}
	indexes, err := d.getIndexes(ctx, conn, table)
	if err != nil {
		return DescribeTableResult{}, err
	}
	return DescribeTableResult{Columns: columns, Indexes: indexes}, nil
}

func (d *SqliteDriver) getColumns(ctx context.Context, conn *sql.DB, table string) ([]ColumnRow, error) {
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns []ColumnRow
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		col := ColumnRow{
			Name:         name,
			DataType:     colType,
			IsNullable:   notNull == 0,
			IsPrimaryKey: pk > 0,
		}
		if dflt.Valid {
			col.DefaultValue = dflt.String
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (d *SqliteDriver) getIndexes(ctx context.Context, conn *sql.DB, table string) ([]IndexRow, error) {
	listRows, err := conn.QueryContext(ctx, `PRAGMA index_list("`+table+`")`)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer listRows.Close()

	type idxMeta struct {
		name   string
		unique bool
	}
	var metas []idxMeta
	for listRows.Next() {
		var seq int
		var name, origin, partial string
		var unique int
		if err := listRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		metas = append(metas, idxMeta{name: name, unique: unique == 1})
	}
	if err := listRows.Err(); err != nil {
		return nil, err
	}

	var indexes []IndexRow
	for _, meta := range metas {
		infoRows, err := conn.QueryContext(ctx, `PRAGMA index_info("`+meta.name+`")`)
		if err != nil {
			return nil, fmt.Errorf("failed to query index info: %v", err)
		}
		var cols []string
		for infoRows.Next() {
			var seqno, cid int
			var colName string
			if err := infoRows.Scan(&seqno, &cid, &colName); err != nil {
				infoRows.Close()
				return nil, fmt.Errorf("scan error: %v", err)
			}
			cols = append(cols, colName)
		}
		infoRows.Close()
		indexes = append(indexes, IndexRow{Name: meta.name, Columns: cols, IsUnique: meta.unique})
	}
	return indexes, nil
}

func (d *SqliteDriver) AnalyzeTable(ctx context.Context, conn *sql.DB, table, schema string) (TableStats, error) {
	stats := TableStats{TableName: table, ColumnStats: make(map[string]string)}

	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+table+`"`).Scan(&stats.RowCount); err != nil {
		return TableStats{}, fmt.Errorf("failed to get row count: %v", err)
	}

	var pageCount, pageSize int64
	_ = conn.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	_ = conn.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
	totalBytes := pageCount * pageSize
	if totalBytes > 0 {
		stats.TableSize = fmt.Sprintf("%.2f MB", float64(totalBytes)/1024/1024)
	} else {
		stats.TableSize = "N/A"
	}
	stats.TotalSize = stats.TableSize
	stats.LastAnalyzed = "N/A"

	colRows, err := conn.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
	if err == nil {
		defer colRows.Close()
		count := 0
		for colRows.Next() && count < 5 {
			var cid, notNull, pk int
			var name, colType string
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err == nil {
				nullability := "Nullable"
				if notNull == 1 {
					nullability = "Not Null"
				}
				stats.ColumnStats[name] = nullability
				count++
			}
		}
	}

	return stats, nil
}

func (d *SqliteDriver) ListViews(ctx context.Context, conn *sql.DB, schema string) ([]ViewRow, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name, 'main' FROM sqlite_master WHERE type = 'view' ORDER BY name`)
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

func (d *SqliteDriver) GetViewDefinition(ctx context.Context, conn *sql.DB, view, schema string) (ViewDefinition, error) {
	def := ViewDefinition{ViewName: view, Schema: "main"}
	err := conn.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'view' AND name = ?`, view).Scan(&def.Definition)
	if err == sql.ErrNoRows {
		return ViewDefinition{}, fmt.Errorf("view '%s' not found", view)
	}
	if err != nil {
		return ViewDefinition{}, fmt.Errorf("query error: %v", err)
	}
	return def, nil
}

func (d *SqliteDriver) ListMaterializedViews(ctx context.Context, conn *sql.DB, schema string) ([]MaterializedViewRow, error) {
	return nil, fmt.Errorf("SQLite does not support materialized views")
}

func (d *SqliteDriver) ListForeignKeys(ctx context.Context, conn *sql.DB, table, schema string) ([]ForeignKeyRow, error) {
	var tables []string
	if table != "" {
		tables = []string{table}
	} else {
		rows, err := conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
	}

	var fks []ForeignKeyRow
	for _, tbl := range tables {
		rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_list("`+tbl+`")`)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		fkMap := make(map[int]*ForeignKeyRow)
		var fkOrder []int
		for rows.Next() {
			var id, seq int
			var refTable, fromCol, toCol, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := fkMap[id]; !ok {
				fkMap[id] = &ForeignKeyRow{
					ConstraintName:  fmt.Sprintf("%s_fk%d", tbl, id),
					TableName:       tbl,
					TableSchema:     "main",
					ReferencedTable: refTable,
					OnDelete:        onDelete,
					OnUpdate:        onUpdate,
				}
				fkOrder = append(fkOrder, id)
			}
			fkMap[id].Columns = append(fkMap[id].Columns, fromCol)
			if toCol != "" {
				fkMap[id].ReferencedColumns = append(fkMap[id].ReferencedColumns, toCol)
			}
		}
		rows.Close()
		for _, id := range fkOrder {
			fks = append(fks, *fkMap[id])
		}
	}
	return fks, nil
}

func (d *SqliteDriver) GetTableRelationships(ctx context.Context, conn *sql.DB, table, schema string) ([]RelationshipRow, error) {
	var rels []RelationshipRow

	// outgoing
	rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_list("`+table+`")`)
	if err != nil {
		return nil, fmt.Errorf("outgoing query error: %v", err)
	}
	relMap := make(map[int]*RelationshipRow)
	var relOrder []int
	for rows.Next() {
		var id, seq int
		var refTable, fromCol, toCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			rows.Close()
			return nil, err
		}
		if _, ok := relMap[id]; !ok {
			relMap[id] = &RelationshipRow{
				Type:           "outgoing",
				ConstraintName: fmt.Sprintf("%s_fk%d", table, id),
				RelatedTable:   refTable,
				RelatedSchema:  "main",
				OnDelete:       onDelete,
				OnUpdate:       onUpdate,
			}
			relOrder = append(relOrder, id)
		}
		relMap[id].Columns = append(relMap[id].Columns, fromCol)
		if toCol != "" {
			relMap[id].RelatedColumns = append(relMap[id].RelatedColumns, toCol)
		}
	}
	rows.Close()
	for _, id := range relOrder {
		rels = append(rels, *relMap[id])
	}

	// incoming: check all other tables for FKs pointing to this table
	allRows, err := conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != ?", table)
	if err != nil {
		return nil, fmt.Errorf("tables query error: %v", err)
	}
	var otherTables []string
	for allRows.Next() {
		var name string
		if err := allRows.Scan(&name); err != nil {
			allRows.Close()
			return nil, err
		}
		otherTables = append(otherTables, name)
	}
	allRows.Close()

	for _, otherTbl := range otherTables {
		fkRows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_list("`+otherTbl+`")`)
		if err != nil {
			continue
		}
		inMap := make(map[int]*RelationshipRow)
		var inOrder []int
		for fkRows.Next() {
			var id, seq int
			var refTable, fromCol, toCol, onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				break
			}
			if refTable != table {
				continue
			}
			if _, ok := inMap[id]; !ok {
				inMap[id] = &RelationshipRow{
					Type:           "incoming",
					ConstraintName: fmt.Sprintf("%s_fk%d", otherTbl, id),
					RelatedTable:   otherTbl,
					RelatedSchema:  "main",
					OnDelete:       onDelete,
					OnUpdate:       onUpdate,
				}
				inOrder = append(inOrder, id)
			}
			if toCol != "" {
				inMap[id].Columns = append(inMap[id].Columns, toCol)
			}
			inMap[id].RelatedColumns = append(inMap[id].RelatedColumns, fromCol)
		}
		fkRows.Close()
		for _, id := range inOrder {
			rels = append(rels, *inMap[id])
		}
	}
	return rels, nil
}

func (d *SqliteDriver) ListTriggers(ctx context.Context, conn *sql.DB, table, schema string) ([]TriggerRow, error) {
	query := `SELECT name, tbl_name, sql FROM sqlite_master WHERE type = 'trigger'`
	var args []interface{}
	if table != "" {
		query += " AND tbl_name = ?"
		args = append(args, table)
	}
	query += " ORDER BY tbl_name, name"

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var triggers []TriggerRow
	for rows.Next() {
		var name, tblName string
		var sqlDef sql.NullString
		if err := rows.Scan(&name, &tblName, &sqlDef); err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}
		event, timing := parseSQLiteTriggerSQL(sqlDef.String)
		triggers = append(triggers, TriggerRow{
			Name:       name,
			Schema:     "main",
			TableName:  tblName,
			Event:      event,
			Timing:     timing,
			ForEachRow: true,
		})
	}
	return triggers, rows.Err()
}

func (d *SqliteDriver) GetTriggerDefinition(ctx context.Context, conn *sql.DB, trigger, schema string) (TriggerDefinition, error) {
	var tblName string
	var sqlDef sql.NullString
	err := conn.QueryRowContext(ctx, `SELECT tbl_name, sql FROM sqlite_master WHERE type = 'trigger' AND name = ?`, trigger).Scan(&tblName, &sqlDef)
	if err == sql.ErrNoRows {
		return TriggerDefinition{}, fmt.Errorf("trigger '%s' not found", trigger)
	}
	if err != nil {
		return TriggerDefinition{}, fmt.Errorf("query error: %v", err)
	}
	event, timing := parseSQLiteTriggerSQL(sqlDef.String)
	return TriggerDefinition{
		TriggerName: trigger,
		Schema:      "main",
		TableName:   tblName,
		Event:       event,
		Timing:      timing,
		Definition:  sqlDef.String,
	}, nil
}

func (d *SqliteDriver) ListFunctions(ctx context.Context, conn *sql.DB, schema string, filterBySchema bool) ([]FunctionRow, error) {
	return nil, fmt.Errorf("SQLite does not support user-defined SQL functions")
}

func (d *SqliteDriver) GetFunctionDefinition(ctx context.Context, conn *sql.DB, fn, schema string) (FunctionDefinition, error) {
	return FunctionDefinition{}, fmt.Errorf("SQLite does not support user-defined SQL functions")
}

func (d *SqliteDriver) ListConstraints(ctx context.Context, conn *sql.DB, table, schema string) ([]ConstraintRow, error) {
	var tables []string
	if table != "" {
		tables = []string{table}
	} else {
		rows, err := conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
	}

	var constraints []ConstraintRow
	for _, tbl := range tables {
		// primary key
		colRows, err := conn.QueryContext(ctx, `PRAGMA table_info("`+tbl+`")`)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		var pkCols []string
		for colRows.Next() {
			var cid, notNull, pk int
			var name, colType string
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
				colRows.Close()
				return nil, err
			}
			if pk > 0 {
				pkCols = append(pkCols, name)
			}
		}
		colRows.Close()
		if len(pkCols) > 0 {
			constraints = append(constraints, ConstraintRow{
				ConstraintName: tbl + "_pkey",
				ConstraintType: "PRIMARY KEY",
				TableName:      tbl,
				TableSchema:    "main",
				Columns:        pkCols,
			})
		}

		// foreign keys
		fkRows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_list("`+tbl+`")`)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		fkMap := make(map[int]*ConstraintRow)
		var fkOrder []int
		for fkRows.Next() {
			var id, seq int
			var refTable, fromCol, toCol, onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				fkRows.Close()
				return nil, err
			}
			if _, ok := fkMap[id]; !ok {
				fkMap[id] = &ConstraintRow{
					ConstraintName:  fmt.Sprintf("%s_fk%d", tbl, id),
					ConstraintType:  "FOREIGN KEY",
					TableName:       tbl,
					TableSchema:     "main",
					ReferencedTable: refTable,
				}
				fkOrder = append(fkOrder, id)
			}
			fkMap[id].Columns = append(fkMap[id].Columns, fromCol)
		}
		fkRows.Close()
		for _, id := range fkOrder {
			constraints = append(constraints, *fkMap[id])
		}

		// unique indexes
		idxRows, err := conn.QueryContext(ctx, `PRAGMA index_list("`+tbl+`")`)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		for idxRows.Next() {
			var seq, unique int
			var name, origin, partial string
			if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				idxRows.Close()
				return nil, err
			}
			if unique == 1 {
				constraints = append(constraints, ConstraintRow{
					ConstraintName: name,
					ConstraintType: "UNIQUE",
					TableName:      tbl,
					TableSchema:    "main",
				})
			}
		}
		idxRows.Close()
	}
	return constraints, nil
}

func (d *SqliteDriver) FindColumns(ctx context.Context, conn *sql.DB, column, table, schema string, exactMatch bool) ([]ColumnMatch, error) {
	var tables []string
	if table != "" {
		tables = []string{table}
	} else {
		rows, err := conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
	}

	search := strings.ToLower(column)
	var cols []ColumnMatch
	for _, tbl := range tables {
		rows, err := conn.QueryContext(ctx, `PRAGMA table_info("`+tbl+`")`)
		if err != nil {
			continue
		}
		for rows.Next() {
			var cid, notNull, pk int
			var name, colType string
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
				break
			}
			nameLower := strings.ToLower(name)
			match := exactMatch && nameLower == search || !exactMatch && strings.Contains(nameLower, search)
			if match {
				cols = append(cols, ColumnMatch{
					TableName:   tbl,
					TableSchema: "main",
					ColumnName:  name,
					DataType:    colType,
					IsNullable:  notNull == 0,
					Position:    cid + 1,
				})
			}
		}
		rows.Close()
	}
	return cols, nil
}

func (d *SqliteDriver) ListSequences(ctx context.Context, conn *sql.DB, schema string) ([]SequenceRow, error) {
	return nil, fmt.Errorf("SQLite does not support sequences")
}

func (d *SqliteDriver) GetSequenceInfo(ctx context.Context, conn *sql.DB, sequence, schema string) (SequenceDetails, error) {
	return SequenceDetails{}, fmt.Errorf("SQLite does not support sequences")
}

func (d *SqliteDriver) ListEnums(ctx context.Context, conn *sql.DB, schema string) ([]EnumRow, error) {
	return nil, fmt.Errorf("SQLite does not support enum types")
}

func (d *SqliteDriver) GetEnumValues(ctx context.Context, conn *sql.DB, enum, schema string) ([]string, error) {
	return nil, fmt.Errorf("SQLite does not support enum types")
}

func parseSQLiteTriggerSQL(s string) (event, timing string) {
	upper := strings.ToUpper(s)
	for _, t := range []string{"INSTEAD OF", "BEFORE", "AFTER"} {
		if strings.Contains(upper, t) {
			timing = t
			break
		}
	}
	for _, e := range []string{"INSERT", "UPDATE", "DELETE"} {
		if strings.Contains(upper, e) {
			event = e
			break
		}
	}
	return event, timing
}
