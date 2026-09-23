package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDKCatalogMatchesContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", "rdl", "stdio_contract.json"))
	if err != nil {
		t.Fatalf("ReadFile(stdio_contract.json) error = %v, want nil", err)
	}
	var baseline struct {
		Handshake []struct {
			Result struct {
				Tools []struct {
					Name        string         `json:"name"`
					Description string         `json:"description"`
					InputSchema map[string]any `json:"inputSchema"`
				} `json:"tools"`
			} `json:"result"`
		} `json:"handshake"`
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
	client := mcp.NewClient(&mcp.Implementation{Name: "catalog-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Client.Connect() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	listed, err := clientSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v, want nil", err)
	}
	if len(listed.Tools) != 15 {
		t.Fatalf("ListTools() count = %d, want 15", len(listed.Tools))
	}
	want := make(map[string]any, len(baseline.Handshake[1].Result.Tools))
	for _, tool := range baseline.Handshake[1].Result.Tools {
		want[tool.Name] = map[string]any{"description": tool.Description, "inputSchema": tool.InputSchema}
	}
	got := make(map[string]any, len(listed.Tools))
	for _, tool := range listed.Tools {
		encoded, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("Marshal(%s.InputSchema) error = %v, want nil", tool.Name, err)
		}
		var schema any
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatalf("Unmarshal(%s.InputSchema) error = %v, want nil", tool.Name, err)
		}
		got[tool.Name] = map[string]any{"description": tool.Description, "inputSchema": schema}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListTools() = %#v, want %#v", got, want)
	}
}
