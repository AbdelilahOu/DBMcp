package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListSchemasInput struct{}

type SchemaInfo struct {
	Name string `json:"name" jsonschema_description:"Schema name"`
}

type ListSchemasOutput struct {
	Schemas []SchemaInfo `json:"schemas" jsonschema_description:"Available schemas in the database"`
}

func GetListSchemasTool() *ToolDefinition[ListSchemasInput, ListSchemasOutput] {
	return NewToolDefinition[ListSchemasInput, ListSchemasOutput](
		"list_schemas",
		"List available schemas (PostgreSQL), databases (MySQL), or attached databases (SQLite). Use this to discover schema names before passing them to other tools.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListSchemasInput) (*mcp.CallToolResult, ListSchemasOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, ListSchemasOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListSchemas(ctx, session.Conn)
			if err != nil {
				logger.LogDatabaseOperation("LIST_SCHEMAS", "list schemas", 0, err)
				return nil, ListSchemasOutput{}, err
			}

			schemas := make([]SchemaInfo, len(rows))
			for i, r := range rows {
				schemas[i] = SchemaInfo{Name: r.Name}
			}

			logger.LogDatabaseOperation("LIST_SCHEMAS", "list schemas", int64(len(schemas)), nil)

			output := ListSchemasOutput{Schemas: schemas}
			message := fmt.Sprintf("Found %d %s", len(schemas), pluralize(len(schemas), "schema", "schemas"))

			return textResult(message), output, nil
		},
	)
}
