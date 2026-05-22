package driver

import (
	"context"
	"database/sql"
)

type DbInfo struct {
	DatabaseName string
	Version      string
	Schemas      []string
	TableCount   int
}

type TableRow struct {
	Name   string
	Schema string
	Type   string
}

type ColumnRow struct {
	Name          string
	DataType      string
	IsNullable    bool
	IsPrimaryKey  bool
	DefaultValue  string
	CharMaxLength *int
}

type IndexRow struct {
	Name     string
	Columns  []string
	IsUnique bool
}

type DescribeTableResult struct {
	Columns []ColumnRow
	Indexes []IndexRow
}

type TableStats struct {
	TableName    string
	RowCount     int64
	TableSize    string
	IndexSize    string
	TotalSize    string
	ColumnStats  map[string]string
	LastAnalyzed string
}

type ViewRow struct {
	Name   string
	Schema string
}

type ViewDefinition struct {
	ViewName   string
	Schema     string
	Definition string
}

type MaterializedViewRow struct {
	Name   string
	Schema string
}

type ForeignKeyRow struct {
	ConstraintName    string
	TableName         string
	TableSchema       string
	Columns           []string
	ReferencedTable   string
	ReferencedSchema  string
	ReferencedColumns []string
	OnDelete          string
	OnUpdate          string
}

type RelationshipRow struct {
	Type           string
	ConstraintName string
	RelatedTable   string
	RelatedSchema  string
	Columns        []string
	RelatedColumns []string
	OnDelete       string
	OnUpdate       string
}

type TriggerRow struct {
	Name       string
	Schema     string
	TableName  string
	Event      string
	Timing     string
	ForEachRow bool
}

type TriggerDefinition struct {
	TriggerName string
	Schema      string
	TableName   string
	Event       string
	Timing      string
	Definition  string
}

type FunctionRow struct {
	Name       string
	Schema     string
	ReturnType string
	Language   string
}

type FunctionDefinition struct {
	FunctionName string
	Schema       string
	ReturnType   string
	Language     string
	Definition   string
	Arguments    string
}

type ConstraintRow struct {
	ConstraintName   string
	ConstraintType   string
	TableName        string
	TableSchema      string
	Columns          []string
	CheckClause      string
	ReferencedTable  string
	ReferencedSchema string
}

type ColumnMatch struct {
	TableName   string
	TableSchema string
	ColumnName  string
	DataType    string
	IsNullable  bool
	Position    int
}

type SequenceRow struct {
	Name   string
	Schema string
}

type SequenceDetails struct {
	SequenceName string
	Schema       string
	LastValue    int64
	StartValue   int64
	IncrementBy  int64
	MinValue     *int64
	MaxValue     *int64
	CacheSize    int64
	Cycle        bool
}

type EnumRow struct {
	Name   string
	Schema string
}

type SchemaRow struct {
	Name string
}

type Driver interface {
	ListSchemas(ctx context.Context, conn *sql.DB) ([]SchemaRow, error)
	GetDbInfo(ctx context.Context, conn *sql.DB) (DbInfo, error)
	ListTables(ctx context.Context, conn *sql.DB, schema string) ([]TableRow, error)
	DescribeTable(ctx context.Context, conn *sql.DB, table, schema string) (DescribeTableResult, error)
	AnalyzeTable(ctx context.Context, conn *sql.DB, table, schema string) (TableStats, error)
	ListViews(ctx context.Context, conn *sql.DB, schema string) ([]ViewRow, error)
	GetViewDefinition(ctx context.Context, conn *sql.DB, view, schema string) (ViewDefinition, error)
	ListMaterializedViews(ctx context.Context, conn *sql.DB, schema string) ([]MaterializedViewRow, error)

	ListForeignKeys(ctx context.Context, conn *sql.DB, table, schema string) ([]ForeignKeyRow, error)
	GetTableRelationships(ctx context.Context, conn *sql.DB, table, schema string) ([]RelationshipRow, error)

	ListTriggers(ctx context.Context, conn *sql.DB, table, schema string) ([]TriggerRow, error)
	GetTriggerDefinition(ctx context.Context, conn *sql.DB, trigger, schema string) (TriggerDefinition, error)

	ListFunctions(ctx context.Context, conn *sql.DB, schema string, filterBySchema bool) ([]FunctionRow, error)
	GetFunctionDefinition(ctx context.Context, conn *sql.DB, fn, schema string) (FunctionDefinition, error)

	ListConstraints(ctx context.Context, conn *sql.DB, table, schema string) ([]ConstraintRow, error)
	FindColumns(ctx context.Context, conn *sql.DB, column, table, schema string, exactMatch bool) ([]ColumnMatch, error)

	ListSequences(ctx context.Context, conn *sql.DB, schema string) ([]SequenceRow, error)
	GetSequenceInfo(ctx context.Context, conn *sql.DB, sequence, schema string) (SequenceDetails, error)

	ListEnums(ctx context.Context, conn *sql.DB, schema string) ([]EnumRow, error)
	GetEnumValues(ctx context.Context, conn *sql.DB, enum, schema string) ([]string, error)

	SupportsEnums() bool
	SupportsSequences() bool
	SupportsMaterializedViews() bool
	SupportsFunctions() bool
	SupportsShowCommands() bool
}
