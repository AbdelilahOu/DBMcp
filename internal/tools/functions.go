package tools

import (
	"context"
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
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, ListFunctionsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListFunctions(ctx, session.Conn, input.Schema, input.Schema != "")
			if err != nil {
				logger.LogDatabaseOperation("LIST_FUNCTIONS", "list functions", 0, err)
				return nil, ListFunctionsOutput{}, err
			}

			functions := make([]FunctionInfo, len(rows))
			for i, r := range rows {
				functions[i] = FunctionInfo{Name: r.Name, Schema: r.Schema, ReturnType: r.ReturnType, Language: r.Language}
			}

			logger.LogDatabaseOperation("LIST_FUNCTIONS", "list functions", int64(len(functions)), nil)

			output := ListFunctionsOutput{Functions: functions}
			message := fmt.Sprintf("Found %d %s", len(functions), pluralize(len(functions), "function", "functions"))
			if input.Schema != "" {
				message += fmt.Sprintf(" in schema '%s'", input.Schema)
			}

			return textResult(message), output, nil
		},
	)
}

func GetFunctionDefinitionTool() *ToolDefinition[GetFunctionDefinitionInput, GetFunctionDefinitionOutput] {
	return NewToolDefinition[GetFunctionDefinitionInput, GetFunctionDefinitionOutput](
		"get_function_definition",
		"Get function source code and definition. Returns signature, params, return type, language, full source. Shows implementation.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetFunctionDefinitionInput) (*mcp.CallToolResult, GetFunctionDefinitionOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, GetFunctionDefinitionOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result, err := session.Driver.GetFunctionDefinition(ctx, session.Conn, input.FunctionName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_FUNCTION_DEFINITION", "get function definition", 0, err)
				return nil, GetFunctionDefinitionOutput{}, err
			}

			logger.LogDatabaseOperation("GET_FUNCTION_DEFINITION", "get function definition", 1, nil)

			output := GetFunctionDefinitionOutput{
				FunctionName: result.FunctionName, Schema: result.Schema, ReturnType: result.ReturnType,
				Language: result.Language, Definition: result.Definition, Arguments: result.Arguments,
			}
			message := fmt.Sprintf("Loaded definition for function %s (%s, returns %s)",
				qualifiedName(output.Schema, output.FunctionName), output.Language, output.ReturnType)

			return textResult(message), output, nil
		},
	)
}
