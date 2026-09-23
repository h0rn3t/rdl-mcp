package mcpserver

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/h0rn3t/rdl-mcp/internal/rdl"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed catalog.json
var catalog embed.FS

type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolArgs struct {
	Filepath            string  `json:"filepath"`
	FieldLimit          int     `json:"field_limit"`
	FieldPattern        *string `json:"field_pattern"`
	OldHeader           string  `json:"old_header"`
	NewHeader           string  `json:"new_header"`
	ColumnIndex         int     `json:"column_index"`
	NewWidth            string  `json:"new_width"`
	FormatString        *string `json:"format_string"`
	HeaderText          string  `json:"header_text"`
	FieldBinding        string  `json:"field_binding"`
	Width               *string `json:"width"`
	FooterExpression    *string `json:"footer_expression"`
	AutoAdjustPageWidth *bool   `json:"auto_adjust_page_width"`
	DatasetName         string  `json:"dataset_name"`
	NewSproc            string  `json:"new_sproc"`
	FieldName           string  `json:"field_name"`
	DataField           string  `json:"data_field"`
	TypeName            string  `json:"type_name"`
	Name                string  `json:"name"`
	DataType            string  `json:"data_type"`
	Prompt              *string `json:"prompt"`
	DefaultValue        *string `json:"default_value"`
}

// New registers the contract-compatible catalog with the official MCP SDK.
func New() (*mcp.Server, error) {
	data, err := catalog.ReadFile("catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded MCP catalog: %w", err)
	}
	var definitions []toolDefinition
	if err := json.Unmarshal(data, &definitions); err != nil {
		return nil, fmt.Errorf("decode MCP catalog: %w", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "rdl-mcp-server", Version: "1.0.0"}, nil)
	var fileMu sync.Mutex
	for _, definition := range definitions {
		mcp.AddTool(server, &mcp.Tool{
			Name:        definition.Name,
			Description: definition.Description,
			InputSchema: definition.InputSchema,
		}, func(_ context.Context, _ *mcp.CallToolRequest, args toolArgs) (*mcp.CallToolResult, any, error) {
			fileMu.Lock()
			defer fileMu.Unlock()
			result, err := runTool(definition.Name, args)
			if err != nil {
				return nil, nil, err
			}
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("encode tool result: %w", err)
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
		})
	}
	return server, nil
}

func runTool(name string, args toolArgs) (map[string]any, error) {
	switch name {
	case "describe_rdl_report":
		return rdl.Describe(args.Filepath)
	case "get_rdl_datasets":
		pattern := ""
		if args.FieldPattern != nil {
			pattern = *args.FieldPattern
		}
		return rdl.Datasets(args.Filepath, args.FieldLimit, pattern)
	case "get_rdl_parameters":
		return rdl.Parameters(args.Filepath)
	case "get_rdl_columns":
		return rdl.Columns(args.Filepath)
	case "validate_rdl":
		return rdl.Validate(args.Filepath), nil
	case "update_column_header":
		return rdl.UpdateColumnHeader(args.Filepath, args.OldHeader, args.NewHeader)
	case "update_column_width":
		return rdl.UpdateColumnWidth(args.Filepath, args.ColumnIndex, args.NewWidth)
	case "update_column_format":
		if args.FormatString == nil {
			return nil, errors.New("format_string is required")
		}
		return rdl.UpdateColumnFormat(args.Filepath, args.ColumnIndex, *args.FormatString)
	case "add_column":
		return rdl.AddColumn(args.Filepath, args.ColumnIndex, args.HeaderText, args.FieldBinding, args.Width, args.FormatString, args.FooterExpression)
	case "remove_column":
		autoAdjust := true
		if args.AutoAdjustPageWidth != nil {
			autoAdjust = *args.AutoAdjustPageWidth
		}
		return rdl.RemoveColumn(args.Filepath, args.ColumnIndex, autoAdjust)
	case "update_stored_procedure":
		return rdl.UpdateStoredProcedure(args.Filepath, args.DatasetName, args.NewSproc)
	case "add_dataset_field":
		return rdl.AddDatasetField(args.Filepath, args.DatasetName, args.FieldName, args.DataField, args.TypeName)
	case "remove_dataset_field":
		return rdl.RemoveDatasetField(args.Filepath, args.DatasetName, args.FieldName)
	case "add_parameter":
		if args.Prompt == nil {
			return nil, errors.New("prompt is required")
		}
		return rdl.AddParameter(args.Filepath, args.Name, args.DataType, *args.Prompt)
	case "update_parameter":
		return rdl.UpdateParameter(args.Filepath, args.Name, args.Prompt, args.DefaultValue)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}
