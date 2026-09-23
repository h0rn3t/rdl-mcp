package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAnalystWorkflowCatalog(t *testing.T) {
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
	client := mcp.NewClient(&mcp.Implementation{Name: "workflow-catalog-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Client.Connect() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	listed, err := clientSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v, want nil", err)
	}
	if len(listed.Tools) != 18 {
		t.Fatalf("ListTools() count = %d, want 18", len(listed.Tools))
	}
	tools := make(map[string]*mcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		tools[tool.Name] = tool
	}
	for _, name := range []string{"get_rdl_textboxes", "compare_rdl_reports", "patch_rdl_textboxes"} {
		if tools[name] == nil {
			t.Errorf("ListTools() missing %q", name)
		}
	}
	for _, name := range []string{
		"get_rdl_columns",
		"update_column_header",
		"update_column_width",
		"update_column_format",
		"add_column",
		"remove_column",
	} {
		tool := tools[name]
		if tool == nil {
			t.Errorf("ListTools() missing %q", name)
			continue
		}
		inputSchema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Errorf("%s.InputSchema = %T, want map", name, tool.InputSchema)
			continue
		}
		properties, ok := inputSchema["properties"].(map[string]any)
		if !ok {
			t.Errorf("%s.InputSchema.properties = %T, want map", name, inputSchema["properties"])
			continue
		}
		tablixName, ok := properties["tablix_name"].(map[string]any)
		if !ok || tablixName["type"] != "string" {
			t.Errorf("%s tablix_name schema = %#v, want optional string property", name, properties["tablix_name"])
		}
		requiredFields, ok := inputSchema["required"].([]any)
		if !ok {
			t.Errorf("%s.InputSchema.required = %T, want []any", name, inputSchema["required"])
			continue
		}
		for _, required := range requiredFields {
			if required == "tablix_name" {
				t.Errorf("%s requires optional tablix_name", name)
			}
		}
	}
	if tool := tools["patch_rdl_textboxes"]; tool != nil {
		inputSchema := tool.InputSchema.(map[string]any)
		properties := inputSchema["properties"].(map[string]any)
		dryRun, ok := properties["dry_run"].(map[string]any)
		if !ok || dryRun["default"] != true {
			t.Errorf("patch_rdl_textboxes dry_run schema = %#v, want default true", properties["dry_run"])
		}
	}
}

func TestAnalystWorkflowToolDispatch(t *testing.T) {
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
	client := mcp.NewClient(&mcp.Implementation{Name: "workflow-dispatch-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Client.Connect() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	path, err := filepath.Abs(filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml"))
	if err != nil {
		t.Fatalf("Abs(workflow_report.xml) error = %v, want nil", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	patches := []any{map[string]any{
		"textbox_name":   "Title",
		"expected_value": "Monthly Revenue",
		"new_value":      "Quarterly Revenue",
	}}
	cases := []struct {
		name string
		args map[string]any
	}{
		{"get_rdl_textboxes", map[string]any{"filepath": path}},
		{"compare_rdl_reports", map[string]any{"filepaths": []string{path, path}}},
		{"patch_rdl_textboxes", map[string]any{"filepaths": []string{path}, "patches": patches}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool(%q) error = %v, want nil", tc.name, err)
			}
			if result.IsError || len(result.Content) != 1 {
				t.Fatalf("CallTool(%q) = %#v, want successful text result", tc.name, result)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("CallTool(%q) content = %T, want text", tc.name, result.Content[0])
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(content.Text), &got); err != nil {
				t.Fatalf("Unmarshal(CallTool(%q)) error = %v, want nil", tc.name, err)
			}
			switch tc.name {
			case "get_rdl_textboxes":
				if textboxes := got["textboxes"].([]any); len(textboxes) != 10 {
					t.Errorf("CallTool(%q) Textboxes count = %d, want 10", tc.name, len(textboxes))
				}
			case "compare_rdl_reports":
				comparisons := got["comparisons"].([]any)
				differences := comparisons[0].(map[string]any)["differences"].(map[string]any)
				for _, category := range []string{"textboxes", "datasets", "parameters", "tablixes", "columns"} {
					if items := differences[category].([]any); len(items) != 0 {
						t.Errorf("CallTool(%q) %s differences = %#v, want none", tc.name, category, items)
					}
				}
			case "patch_rdl_textboxes":
				if got["success"] != true || got["dry_run"] != true {
					t.Errorf("CallTool(%q) = %#v, want successful dry-run by default", tc.name, got)
				}
				files := got["files"].([]any)
				if files[0].(map[string]any)["matched_count"] != float64(1) {
					t.Errorf("CallTool(%q) matched_count = %#v, want 1", tc.name, files[0])
				}
			}
		})
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) after SDK calls error = %v, want nil", path, err)
	}
	if !slices.Equal(after, before) {
		t.Errorf("read, compare, and default patch preview changed %q", path)
	}
}
