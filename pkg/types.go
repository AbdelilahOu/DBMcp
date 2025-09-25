package mcpdb

type ExecuteSelectInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SELECT SQL query to execute (e.g. 'SELECT * FROM users LIMIT 5')"`
}

type ExecuteSelectOutput struct {
	Results string `json:"results" jsonschema_description:"JSON array of query results"`
}

type ListTablesInput struct {
	Schema string `json:"schema,omitempty" jsonschema_description:"Optional schema name to filter tables (defaults to 'public' for PostgreSQL)"`
}

type TableInfo struct {
	Name   string `json:"name" jsonschema_description:"Table name"`
	Schema string `json:"schema" jsonschema_description:"Schema name"`
	Type   string `json:"type" jsonschema_description:"Table type (table, view, etc.)"`
}

type ListTablesOutput struct {
	Tables []TableInfo `json:"tables" jsonschema_description:"Array of table information"`
}

type DescribeTableInput struct {
	TableName string `json:"table_name" jsonschema:"required" jsonschema_description:"Name of the table to describe"`
	Schema    string `json:"schema,omitempty" jsonschema_description:"Optional schema name (defaults to 'public' for PostgreSQL)"`
}

type ColumnInfo struct {
	Name          string `json:"name" jsonschema_description:"Column name"`
	DataType      string `json:"data_type" jsonschema_description:"Data type of the column"`
	IsNullable    bool   `json:"is_nullable" jsonschema_description:"Whether the column can contain NULL values"`
	IsPrimaryKey  bool   `json:"is_primary_key" jsonschema_description:"Whether the column is part of the primary key"`
	DefaultValue  string `json:"default_value,omitempty" jsonschema_description:"Default value for the column"`
	CharMaxLength *int   `json:"char_max_length,omitempty" jsonschema_description:"Maximum length for character types"`
}

type IndexInfo struct {
	Name     string   `json:"name" jsonschema_description:"Index name"`
	Columns  []string `json:"columns" jsonschema_description:"Columns included in the index"`
	IsUnique bool     `json:"is_unique" jsonschema_description:"Whether the index is unique"`
}

type DescribeTableOutput struct {
	Columns []ColumnInfo `json:"columns" jsonschema_description:"Array of column information"`
	Indexes []IndexInfo  `json:"indexes" jsonschema_description:"Array of index information"`
}

type GetDBInfoInput struct{}

type GetDBInfoOutput struct {
	DatabaseName string   `json:"database_name" jsonschema_description:"Name of the database"`
	Version      string   `json:"version" jsonschema_description:"Database version"`
	Schemas      []string `json:"schemas" jsonschema_description:"Available schemas"`
	TableCount   int      `json:"table_count" jsonschema_description:"Total number of tables"`
}

type ExecuteQueryInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SQL query to execute (INSERT, UPDATE, DELETE, etc.)"`
}

type ExecuteQueryOutput struct {
	RowsAffected int64  `json:"rows_affected" jsonschema_description:"Number of rows affected by the query"`
	Message      string `json:"message" jsonschema_description:"Success message"`
}

type ExplainQueryInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"SQL query to explain"`
}

type ExplainQueryOutput struct {
	Plan string `json:"plan" jsonschema_description:"Query execution plan"`
}

type ListConnectionsInput struct{}

type ConnectionInfo struct {
	Name        string `json:"name" jsonschema_description:"Connection name"`
	DisplayName string `json:"display_name" jsonschema_description:"Human-readable connection name"`
	Type        string `json:"type" jsonschema_description:"Database type (postgres, mysql)"`
	Description string `json:"description" jsonschema_description:"Connection description"`
}

type ListConnectionsOutput struct {
	Connections       []ConnectionInfo `json:"connections" jsonschema_description:"Available connections"`
	DefaultConnection string           `json:"default_connection" jsonschema_description:"Default connection name"`
}

type SwitchConnectionInput struct {
	Connection string `json:"connection" jsonschema:"required" jsonschema_description:"Name of the connection to switch to"`
}

type SwitchConnectionOutput struct {
	Message    string `json:"message" jsonschema_description:"Success message"`
	Connection string `json:"connection" jsonschema_description:"Active connection name"`
}

type TestConnectionInput struct {
	Connection string `json:"connection,omitempty" jsonschema_description:"Optional connection name to test (uses current if not specified)"`
}

type TestConnectionOutput struct {
	Success    bool   `json:"success" jsonschema_description:"Whether the connection test succeeded"`
	Message    string `json:"message" jsonschema_description:"Test result message"`
	Connection string `json:"connection" jsonschema_description:"Connection that was tested"`
}

type AnalyzeTableInput struct {
	TableName string `json:"table_name" jsonschema:"required" jsonschema_description:"Name of the table to analyze"`
	Schema    string `json:"schema,omitempty" jsonschema_description:"Optional schema name"`
}

type TableStats struct {
	TableName    string            `json:"table_name" jsonschema_description:"Table name"`
	RowCount     int64             `json:"row_count" jsonschema_description:"Approximate number of rows"`
	TableSize    string            `json:"table_size" jsonschema_description:"Table size in human-readable format"`
	IndexSize    string            `json:"index_size" jsonschema_description:"Index size in human-readable format"`
	TotalSize    string            `json:"total_size" jsonschema_description:"Total size (table + indexes)"`
	ColumnStats  map[string]string `json:"column_stats,omitempty" jsonschema_description:"Basic statistics for columns"`
	LastAnalyzed string            `json:"last_analyzed,omitempty" jsonschema_description:"When the table was last analyzed"`
}

type AnalyzeTableOutput struct {
	Stats TableStats `json:"stats" jsonschema_description:"Table statistics"`
}
