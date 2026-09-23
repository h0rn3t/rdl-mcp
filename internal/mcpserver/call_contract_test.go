package mcpserver

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDKCallsMatchStdioContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", "rdl", "stdio_contract.json"))
	if err != nil {
		t.Fatalf("ReadFile(stdio_contract.json) error = %v, want nil", err)
	}
	var baseline struct {
		Cases map[string]struct {
			Result struct {
				Content []struct {
					Text any `json:"text"`
				} `json:"content"`
			} `json:"result"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("Unmarshal(stdio_contract.json) error = %v, want nil", err)
	}
	server, err := New()
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("Server.Connect() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "call-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Client.Connect() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	cases := []struct {
		name string
		args map[string]any
	}{
		{"describe_rdl_report", nil},
		{"get_rdl_datasets", nil},
		{"get_rdl_parameters", nil},
		{"get_rdl_columns", nil},
		{"validate_rdl", nil},
		{"update_column_header", map[string]any{"old_header": "Name", "new_header": "Full Name"}},
		{"update_column_width", map[string]any{"column_index": 0, "new_width": "1.5in"}},
		{"update_column_format", map[string]any{"column_index": 2, "format_string": "C2"}},
		{"add_column", map[string]any{"column_index": 1, "header_text": "New", "field_binding": "Name"}},
		{"remove_column", map[string]any{"column_index": 1}},
		{"update_stored_procedure", map[string]any{"dataset_name": "MainDataset", "new_sproc": "usp_NewProcedure"}},
		{"add_dataset_field", map[string]any{"dataset_name": "MainDataset", "field_name": "NewField", "data_field": "NewField", "type_name": "System.String"}},
		{"remove_dataset_field", map[string]any{"dataset_name": "MainDataset", "field_name": "CreatedDate"}},
		{"add_parameter", map[string]any{"name": "NewParam", "data_type": "String", "prompt": "New parameter"}},
		{"update_parameter", map[string]any{"name": "StartDate", "prompt": "Updated start"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.rdl")
			report, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", "rdl", "sample_report.xml"))
			if err != nil {
				t.Fatalf("ReadFile(sample_report.xml) error = %v, want nil", err)
			}
			if err := os.WriteFile(path, report, 0o600); err != nil {
				t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
			}
			args := map[string]any{"filepath": path}
			maps.Copy(args, tc.args)
			result, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool(%q) error = %v, want nil", tc.name, err)
			}
			if result.IsError || len(result.Content) != 1 {
				t.Fatalf("CallTool(%q) = %#v, want one successful text content", tc.name, result)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("CallTool(%q) content = %T, want *mcp.TextContent", tc.name, result.Content[0])
			}
			var got any
			if err := json.Unmarshal([]byte(content.Text), &got); err != nil {
				t.Fatalf("Unmarshal(CallTool(%q).Text) error = %v, want nil", tc.name, err)
			}
			if object, ok := got.(map[string]any); ok && object["filepath"] == path {
				object["filepath"] = "<RDL>"
			}
			want := baseline.Cases[tc.name].Result.Content[0].Text
			if !reflect.DeepEqual(got, want) {
				t.Errorf("CallTool(%q) = %#v, want %#v", tc.name, got, want)
			}
		})
	}
}
