package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListFunctionsInput struct {
	Schema string `json:"sch,omitempty" jsonschema_description:"Optional schema name to filter functions"`
}

type FunctionInfo struct {
	Name       string `json:"name" jsonschema_description:"Function name"`
	Schema     string `json:"sch" jsonschema_description:"Schema name"`
	ReturnType string `json:"rtype" jsonschema_description:"Return type of the function"`
	Language   string `json:"lang" jsonschema_description:"Language the function is written in (SQL, plpgsql, etc.)"`
}

type ListFunctionsOutput struct {
	Functions []FunctionInfo `json:"functions" jsonschema_description:"Array of function information"`
}

type GetFunctionDefinitionInput struct {
	FunctionName string `json:"fn" jsonschema:"required" jsonschema_description:"Name of the function"`
	Schema       string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type GetFunctionDefinitionOutput struct {
	FunctionName string `json:"fn" jsonschema_description:"Name of the function"`
	Schema       string `json:"sch" jsonschema_description:"Schema of the function"`
	ReturnType   string `json:"rtype" jsonschema_description:"Return type"`
	Language     string `json:"lang" jsonschema_description:"Language the function is written in"`
	Definition   string `json:"def" jsonschema_description:"Complete function definition/source code"`
	Arguments    string `json:"args,omitempty" jsonschema_description:"Function arguments/parameters"`
}

func GetListFunctionsTool() *ToolDefinition[ListFunctionsInput, ListFunctionsOutput] {
	return NewToolDefinition[ListFunctionsInput, ListFunctionsOutput](
		"list_functions",
		"List user-defined functions/procedures. Returns names, schemas, return types, language. Use get_function_definition for source code.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListFunctionsInput) (*mcp.CallToolResult, ListFunctionsOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, ListFunctionsOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, ListFunctionsOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var functions []FunctionInfo
			var err2 error

			if sessionState.DBType == "postgres" {
				functions, err2 = getPostgresFunctions(ctx, sessionState.Conn, schema, input.Schema != "")
			} else {
				functions, err2 = getMySQLFunctions(ctx, sessionState.Conn, schema, input.Schema != "")
			}

			if err2 != nil {
				logger.LogDatabaseOperation("LIST_FUNCTIONS", "list functions", 0, err2)
				return nil, ListFunctionsOutput{}, err2
			}

			logger.LogDatabaseOperation("LIST_FUNCTIONS", "list functions", int64(len(functions)), nil)

			output := ListFunctionsOutput{Functions: functions}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, ListFunctionsOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func GetFunctionDefinitionTool() *ToolDefinition[GetFunctionDefinitionInput, GetFunctionDefinitionOutput] {
	return NewToolDefinition[GetFunctionDefinitionInput, GetFunctionDefinitionOutput](
		"get_function_definition",
		"Get function source code and definition. Returns signature, params, return type, language, full source. Shows implementation.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetFunctionDefinitionInput) (*mcp.CallToolResult, GetFunctionDefinitionOutput, error) {
			sessionState, err := state.GetActiveSession("default")
			if err != nil {
				return nil, GetFunctionDefinitionOutput{}, err
			}

			if sessionState.DBType != "postgres" && sessionState.DBType != "mysql" {
				return nil, GetFunctionDefinitionOutput{}, fmt.Errorf("unsupported database type: %s. Only 'postgres' and 'mysql' are supported", sessionState.DBType)
			}

			schema := input.Schema
			if schema == "" {
				schema = sessionState.CurrentSchema
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var output GetFunctionDefinitionOutput
			var err2 error

			if sessionState.DBType == "postgres" {
				output, err2 = getPostgresFunctionDefinition(ctx, sessionState.Conn, input.FunctionName, schema)
			} else {
				output, err2 = getMySQLFunctionDefinition(ctx, sessionState.Conn, input.FunctionName, schema)
			}

			if err2 != nil {
				logger.LogDatabaseOperation("GET_FUNCTION_DEFINITION", "get function definition", 0, err2)
				return nil, GetFunctionDefinitionOutput{}, err2
			}

			logger.LogDatabaseOperation("GET_FUNCTION_DEFINITION", "get function definition", 1, nil)

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				return nil, GetFunctionDefinitionOutput{}, fmt.Errorf("JSON marshal error: %v", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonBytes)},
				},
			}, output, nil
		},
	)
}

func getPostgresFunctions(ctx context.Context, conn *sql.DB, schema string, hasSchemaFilter bool) ([]FunctionInfo, error) {
	query := `
		SELECT
			p.proname as function_name,
			n.nspname as schema_name,
			pg_catalog.pg_get_function_result(p.oid) as return_type,
			l.lanname as language
		FROM pg_proc p
		JOIN pg_namespace n ON p.pronamespace = n.oid
		JOIN pg_language l ON p.prolang = l.oid
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')`

	if hasSchemaFilter {
		query += " AND n.nspname = $1"
	}

	query += " ORDER BY n.nspname, p.proname"

	var rows *sql.Rows
	var err error

	if hasSchemaFilter {
		rows, err = conn.QueryContext(ctx, query, schema)
	} else {
		rows, err = conn.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var functions []FunctionInfo
	for rows.Next() {
		var fn FunctionInfo

		err := rows.Scan(
			&fn.Name,
			&fn.Schema,
			&fn.ReturnType,
			&fn.Language,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		functions = append(functions, fn)
	}

	return functions, rows.Err()
}

func getMySQLFunctions(ctx context.Context, conn *sql.DB, schema string, hasSchemaFilter bool) ([]FunctionInfo, error) {
	query := `
		SELECT
			routine_name as function_name,
			routine_schema as schema_name,
			COALESCE(dtd_identifier, 'void') as return_type,
			'SQL' as language
		FROM information_schema.routines
		WHERE routine_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')`

	if hasSchemaFilter {
		query += " AND routine_schema = ?"
	}

	query += " ORDER BY routine_schema, routine_name"

	var rows *sql.Rows
	var err error

	if hasSchemaFilter {
		rows, err = conn.QueryContext(ctx, query, schema)
	} else {
		rows, err = conn.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var functions []FunctionInfo
	for rows.Next() {
		var fn FunctionInfo

		err := rows.Scan(
			&fn.Name,
			&fn.Schema,
			&fn.ReturnType,
			&fn.Language,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %v", err)
		}

		functions = append(functions, fn)
	}

	return functions, rows.Err()
}

func getPostgresFunctionDefinition(ctx context.Context, conn *sql.DB, functionName, schema string) (GetFunctionDefinitionOutput, error) {
	query := `
		SELECT
			p.proname as function_name,
			n.nspname as schema_name,
			pg_catalog.pg_get_function_result(p.oid) as return_type,
			l.lanname as language,
			pg_catalog.pg_get_functiondef(p.oid) as definition,
			pg_catalog.pg_get_function_arguments(p.oid) as arguments
		FROM pg_proc p
		JOIN pg_namespace n ON p.pronamespace = n.oid
		JOIN pg_language l ON p.prolang = l.oid
		WHERE p.proname = $1 AND n.nspname = $2`

	var output GetFunctionDefinitionOutput

	err := conn.QueryRowContext(ctx, query, functionName, schema).Scan(
		&output.FunctionName,
		&output.Schema,
		&output.ReturnType,
		&output.Language,
		&output.Definition,
		&output.Arguments,
	)

	if err == sql.ErrNoRows {
		return GetFunctionDefinitionOutput{}, fmt.Errorf("function '%s' not found in schema '%s'", functionName, schema)
	}
	if err != nil {
		return GetFunctionDefinitionOutput{}, fmt.Errorf("query error: %v", err)
	}

	return output, nil
}

func getMySQLFunctionDefinition(ctx context.Context, conn *sql.DB, functionName, schema string) (GetFunctionDefinitionOutput, error) {
	query := `
		SELECT
			routine_name as function_name,
			routine_schema as schema_name,
			COALESCE(dtd_identifier, 'void') as return_type,
			'SQL' as language,
			routine_definition as definition,
			COALESCE(parameter_style, '') as arguments
		FROM information_schema.routines
		WHERE routine_name = ? AND routine_schema = ?`

	var output GetFunctionDefinitionOutput

	err := conn.QueryRowContext(ctx, query, functionName, schema).Scan(
		&output.FunctionName,
		&output.Schema,
		&output.ReturnType,
		&output.Language,
		&output.Definition,
		&output.Arguments,
	)

	if err == sql.ErrNoRows {
		return GetFunctionDefinitionOutput{}, fmt.Errorf("function '%s' not found in schema '%s'", functionName, schema)
	}
	if err != nil {
		return GetFunctionDefinitionOutput{}, fmt.Errorf("query error: %v", err)
	}

	return output, nil
}
