package driver

import (
	"context"
	"database/sql"
)

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
