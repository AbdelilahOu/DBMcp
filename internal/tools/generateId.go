package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lucsky/cuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	cuid2 "github.com/nrednav/cuid2"
)

type GenerateIdInput struct {
	Type      string `json:"type" jsonschema:"required" jsonschema_description:"Type of ID to generate: 'uuid_v1', 'uuid_v3', 'uuid_v4', 'uuid_v5', 'uuid_v6', 'uuid_v7', 'cuid', 'cuid2'"`
	Count     int    `json:"count,omitempty" jsonschema_description:"Number of IDs to generate (default: 1, max: 100)"`
	Namespace string `json:"namespace,omitempty" jsonschema_description:"Namespace for UUID v3/v5 (dns, url, oid, x500, or a valid UUID string)"`
	Name      string `json:"name,omitempty" jsonschema_description:"Name string for UUID v3/v5"`
}

type GenerateIdOutput struct {
	IDs     []string `json:"ids" jsonschema_description:"Generated IDs"`
	Message string   `json:"msg" jsonschema_description:"Success message"`
}

func GetGenerateIdTool() *ToolDefinition[GenerateIdInput, GenerateIdOutput] {
	return NewToolDefinition[GenerateIdInput, GenerateIdOutput](
		"generate_id",
		"Generate unique IDs: UUID v1-v7, CUID. UUID: v1(timestamp+MAC), v3(MD5), v4(random), v5(SHA-1), v6(reordered time), v7(Unix time). CUID: collision-resistant, horizontal scaling.",
		func(ctx context.Context, req *mcp.CallToolRequest, input GenerateIdInput) (*mcp.CallToolResult, GenerateIdOutput, error) {
			if input.Count <= 0 {
				input.Count = 1
			}
			if input.Count > 100 {
				return nil, GenerateIdOutput{}, fmt.Errorf("count cannot exceed 100")
			}

			ids := make([]string, 0, input.Count)
			var err error

			for i := 0; i < input.Count; i++ {
				var id string
				switch strings.ToLower(input.Type) {
				case "v1":
				case "uuid-v1":
				case "uuid_v1":
					id, err = generateUUIDv1()
				case "v3":
				case "uuid-v3":
				case "uuid_v3":
					if input.Namespace == "" || input.Name == "" {
						return nil, GenerateIdOutput{}, fmt.Errorf("namespace and name are required for UUID v3")
					}
					id, err = generateUUIDv3(input.Namespace, input.Name)
				case "v4":
				case "uuid-v4":
				case "uuid_v4":
					id, err = generateUUIDv4()
				case "v5":
				case "uuid-v5":
				case "uuid_v5":
					if input.Namespace == "" || input.Name == "" {
						return nil, GenerateIdOutput{}, fmt.Errorf("namespace and name are required for UUID v5")
					}
					id, err = generateUUIDv5(input.Namespace, input.Name)
				case "v6":
				case "uuid-v6":
				case "uuid_v6":
					id, err = generateUUIDv6()
				case "v7":
				case "uuid-v7":
				case "uuid_v7":
					id, err = generateUUIDv7()
				case "cuid2":
					id = cuid2.Generate()
				case "cuid":
					id = cuid.New()
				default:
					return nil, GenerateIdOutput{}, fmt.Errorf("unsupported ID type: %s. Supported types: uuid_v1, uuid_v3, uuid_v4, uuid_v5, uuid_v6, uuid_v7, cuid, cuid2", input.Type)
				}

				if err != nil {
					return nil, GenerateIdOutput{}, fmt.Errorf("error generating %s: %v", input.Type, err)
				}

				ids = append(ids, id)
			}

			output := GenerateIdOutput{
				IDs:     ids,
				Message: fmt.Sprintf("Successfully generated %d %s ID(s)", input.Count, input.Type),
			}

			return textResult(output.Message), output, nil
		},
	)
}

func generateUUIDv1() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func generateUUIDv3(namespace, name string) (string, error) {
	ns, err := parseNamespace(namespace)
	if err != nil {
		return "", err
	}
	return uuid.NewMD5(ns, []byte(name)).String(), nil
}

func generateUUIDv4() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func generateUUIDv5(namespace, name string) (string, error) {
	ns, err := parseNamespace(namespace)
	if err != nil {
		return "", err
	}
	return uuid.NewSHA1(ns, []byte(name)).String(), nil
}

func generateUUIDv6() (string, error) {
	id, err := uuid.NewV6()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func generateUUIDv7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func parseNamespace(ns string) (uuid.UUID, error) {
	switch ns {
	case "dns":
		return uuid.NameSpaceDNS, nil
	case "url":
		return uuid.NameSpaceURL, nil
	case "oid":
		return uuid.NameSpaceOID, nil
	case "x500":
		return uuid.NameSpaceX500, nil
	default:
		parsed, err := uuid.Parse(ns)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid namespace: must be 'dns', 'url', 'oid', 'x500', or a valid UUID")
		}
		return parsed, nil
	}
}
