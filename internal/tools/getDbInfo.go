package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetDBInfoInput struct{}

type GetDBInfoOutput struct {
	DatabaseName string   `json:"db" jsonschema_description:"Name of the database"`
	Version      string   `json:"version" jsonschema_description:"Database version"`
	Schemas      []string `json:"schemas" jsonschema_description:"Available schemas"`
	TableCount   int      `json:"table_count" jsonschema_description:"Total number of tables"`
}

func GetDbInfoTool() *ToolDefinition[GetDBInfoInput, GetDBInfoOutput] {
	return NewToolDefinition[GetDBInfoInput, GetDBInfoOutput](
		"get_db_info",
		"Get DB overview: name, version, schemas, table count. Entry point for unfamiliar DBs. Use list_tables/describe_table/analyze_table for table details.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetDBInfoInput) (*mcp.CallToolResult, GetDBInfoOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, GetDBInfoOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			info, err := session.Driver.GetDbInfo(ctx, session.Conn)
			if err != nil {
				logger.LogDatabaseOperation("GET_DB_INFO", "get db info", 0, err)
				return nil, GetDBInfoOutput{}, err
			}

			output := GetDBInfoOutput{
				DatabaseName: info.DatabaseName,
				Version:      info.Version,
				Schemas:      info.Schemas,
				TableCount:   info.TableCount,
			}

			logger.LogDatabaseOperation("GET_DB_INFO", "Database information query", int64(output.TableCount), nil)

			message := fmt.Sprintf(
				"Database '%s' (%s): %d %s, %d %s",
				output.DatabaseName, output.Version,
				len(output.Schemas), pluralize(len(output.Schemas), "schema", "schemas"),
				output.TableCount, pluralize(output.TableCount, "table", "tables"),
			)

			return textResult(message), output, nil
		},
	)
}
