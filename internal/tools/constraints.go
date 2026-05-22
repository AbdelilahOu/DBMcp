package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListConstraintsInput struct {
	TableName string `json:"tbl,omitempty" jsonschema_description:"Optional table name to filter constraints"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type ConstraintInfo struct {
	ConstraintName   string   `json:"cname" jsonschema_description:"Name of the constraint"`
	ConstraintType   string   `json:"constraint_type" jsonschema_description:"Type of constraint (PRIMARY KEY, FOREIGN KEY, UNIQUE, CHECK)"`
	TableName        string   `json:"tbl" jsonschema_description:"Table the constraint is on"`
	TableSchema      string   `json:"sch" jsonschema_description:"Schema of the table"`
	Columns          []string `json:"columns" jsonschema_description:"Columns involved in the constraint"`
	CheckClause      string   `json:"check_clause,omitempty" jsonschema_description:"CHECK constraint expression (if applicable)"`
	ReferencedTable  string   `json:"referenced_table,omitempty" jsonschema_description:"Referenced table (for foreign keys)"`
	ReferencedSchema string   `json:"referenced_schema,omitempty" jsonschema_description:"Referenced schema (for foreign keys)"`
}

type ListConstraintsOutput struct {
	Constraints []ConstraintInfo `json:"constraints" jsonschema_description:"Array of constraint information"`
}

func GetListConstraintsTool() *ToolDefinition[ListConstraintsInput, ListConstraintsOutput] {
	return NewToolDefinition[ListConstraintsInput, ListConstraintsOutput](
		"list_constraints",
		"List constraints in DB or table. Returns PKs, FKs, unique, check constraints. Shows data integrity rules. Use list_foreign_keys for detailed FK info.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListConstraintsInput) (*mcp.CallToolResult, ListConstraintsOutput, error) {
			session, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListConstraintsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			rows, err := session.Driver.ListConstraints(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_CONSTRAINTS", "list constraints", 0, err)
				return nil, ListConstraintsOutput{}, err
			}

			constraints := make([]ConstraintInfo, len(rows))
			for i, r := range rows {
				constraints[i] = ConstraintInfo{
					ConstraintName:   r.ConstraintName,
					ConstraintType:   r.ConstraintType,
					TableName:        r.TableName,
					TableSchema:      r.TableSchema,
					Columns:          r.Columns,
					CheckClause:      r.CheckClause,
					ReferencedTable:  r.ReferencedTable,
					ReferencedSchema: r.ReferencedSchema,
				}
			}

			logger.LogDatabaseOperation("LIST_CONSTRAINTS", "list constraints", int64(len(constraints)), nil)

			output := ListConstraintsOutput{Constraints: constraints}
			message := fmt.Sprintf("Found %d %s", len(constraints), pluralize(len(constraints), "constraint", "constraints"))
			if input.TableName != "" {
				message += fmt.Sprintf(" for %s", qualifiedName(input.Schema, input.TableName))
			}

			return textResult(message), output, nil
		},
	)
}
