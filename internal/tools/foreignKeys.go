package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/logger"
	"github.com/AbdelilahOu/DBMcp/internal/state"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListForeignKeysInput struct {
	TableName string `json:"tbl,omitempty" jsonschema_description:"Optional table name to filter foreign keys"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type ForeignKeyInfo struct {
	ConstraintName    string   `json:"cname" jsonschema_description:"Name of the foreign key constraint"`
	TableName         string   `json:"tbl" jsonschema_description:"Table containing the foreign key"`
	TableSchema       string   `json:"sch" jsonschema_description:"Schema of the table"`
	Columns           []string `json:"cols" jsonschema_description:"Columns in the foreign key"`
	ReferencedTable   string   `json:"ref_tbl" jsonschema_description:"Table being referenced"`
	ReferencedSchema  string   `json:"ref_sch,omitempty" jsonschema_description:"Schema of the referenced table"`
	ReferencedColumns []string `json:"ref_cols" jsonschema_description:"Columns being referenced"`
	OnDelete          string   `json:"on_del,omitempty" jsonschema_description:"Action on delete (CASCADE, SET NULL, etc.)"`
	OnUpdate          string   `json:"on_upd,omitempty" jsonschema_description:"Action on update (CASCADE, SET NULL, etc.)"`
}

type ListForeignKeysOutput struct {
	ForeignKeys []ForeignKeyInfo `json:"fks" jsonschema_description:"Array of foreign key information"`
}

type GetTableRelationshipsInput struct {
	TableName string `json:"tbl" jsonschema:"required" jsonschema_description:"Name of the table"`
	Schema    string `json:"sch,omitempty" jsonschema_description:"Optional schema name"`
}

type RelationshipInfo struct {
	Type           string   `json:"type" jsonschema_description:"Relationship type: 'outgoing' or 'incoming'"`
	ConstraintName string   `json:"cname" jsonschema_description:"Name of the foreign key constraint"`
	RelatedTable   string   `json:"rel_tbl" jsonschema_description:"The other table in the relationship"`
	RelatedSchema  string   `json:"rel_sch,omitempty" jsonschema_description:"Schema of the related table"`
	Columns        []string `json:"cols" jsonschema_description:"Columns involved in this table"`
	RelatedColumns []string `json:"rel_cols" jsonschema_description:"Columns in the related table"`
	OnDelete       string   `json:"on_del,omitempty" jsonschema_description:"Action on delete"`
	OnUpdate       string   `json:"on_upd,omitempty" jsonschema_description:"Action on update"`
}

type GetTableRelationshipsOutput struct {
	TableName     string             `json:"tbl" jsonschema_description:"Name of the table"`
	TableSchema   string             `json:"sch" jsonschema_description:"Schema of the table"`
	Relationships []RelationshipInfo `json:"rels" jsonschema_description:"Array of relationships (both incoming and outgoing)"`
}

func GetListForeignKeysTool() *ToolDefinition[ListForeignKeysInput, ListForeignKeysOutput] {
	return NewToolDefinition[ListForeignKeysInput, ListForeignKeysOutput](
		"list_foreign_keys",
		"List FK constraints in DB or table. Returns columns, referenced tables, actions (CASCADE, SET NULL, etc.). Shows relationships and referential integrity.",
		func(ctx context.Context, req *mcp.CallToolRequest, input ListForeignKeysInput) (*mcp.CallToolResult, ListForeignKeysOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, ListForeignKeysOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.ListForeignKeys(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("LIST_FOREIGN_KEYS", "list foreign keys", 0, err)
				return nil, ListForeignKeysOutput{}, err
			}

			fks := make([]ForeignKeyInfo, len(rows))
			for i, r := range rows {
				fks[i] = ForeignKeyInfo{
					ConstraintName:    r.ConstraintName,
					TableName:         r.TableName,
					TableSchema:       r.TableSchema,
					Columns:           r.Columns,
					ReferencedTable:   r.ReferencedTable,
					ReferencedSchema:  r.ReferencedSchema,
					ReferencedColumns: r.ReferencedColumns,
					OnDelete:          r.OnDelete,
					OnUpdate:          r.OnUpdate,
				}
			}

			logger.LogDatabaseOperation("LIST_FOREIGN_KEYS", "list foreign keys", int64(len(fks)), nil)

			output := ListForeignKeysOutput{ForeignKeys: fks}
			message := fmt.Sprintf("Found %d foreign %s", len(fks), pluralize(len(fks), "key", "keys"))
			if input.TableName != "" {
				message += fmt.Sprintf(" for %s", qualifiedName(input.Schema, input.TableName))
			}

			return textResult(message), output, nil
		},
	)
}

func GetTableRelationshipsTool() *ToolDefinition[GetTableRelationshipsInput, GetTableRelationshipsOutput] {
	return NewToolDefinition[GetTableRelationshipsInput, GetTableRelationshipsOutput](
		"get_table_relationships",
		"Get all relationships for table: outgoing FKs (refs other tables) and incoming FKs (other tables ref this). Complete relationship view.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GetTableRelationshipsInput) (*mcp.CallToolResult, GetTableRelationshipsOutput, error) {
			session, err := state.GetActiveSession()
			if err != nil {
				return nil, GetTableRelationshipsOutput{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			rows, err := session.Driver.GetTableRelationships(ctx, session.Conn, input.TableName, input.Schema)
			if err != nil {
				logger.LogDatabaseOperation("GET_TABLE_RELATIONSHIPS", fmt.Sprintf("get relationships for %s", input.TableName), 0, err)
				return nil, GetTableRelationshipsOutput{}, err
			}

			rels := make([]RelationshipInfo, len(rows))
			for i, r := range rows {
				rels[i] = RelationshipInfo{
					Type:           r.Type,
					ConstraintName: r.ConstraintName,
					RelatedTable:   r.RelatedTable,
					RelatedSchema:  r.RelatedSchema,
					Columns:        r.Columns,
					RelatedColumns: r.RelatedColumns,
					OnDelete:       r.OnDelete,
					OnUpdate:       r.OnUpdate,
				}
			}

			logger.LogDatabaseOperation("GET_TABLE_RELATIONSHIPS", fmt.Sprintf("get relationships for %s", input.TableName), int64(len(rels)), nil)

			output := GetTableRelationshipsOutput{TableName: input.TableName, TableSchema: input.Schema, Relationships: rels}
			message := fmt.Sprintf("Found %d %s for %s",
				len(rels), pluralize(len(rels), "relationship", "relationships"),
				qualifiedName(input.Schema, input.TableName))

			return textResult(message), output, nil
		},
	)
}
