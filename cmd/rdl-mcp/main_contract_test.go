package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStdioSessionAndEOF(t *testing.T) {
	testStdioSession(t, false)
}

func TestMalformedJSONEndsSession(t *testing.T) {
	testStdioSession(t, true)
}

func testStdioSession(t *testing.T, malformed bool) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "rdl-mcp")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -o %q . error = %v, output = %s", binary, err, output)
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "tests", "baseline", "sample_report.xml"))
	if err != nil {
		t.Fatalf("Abs(sample_report.xml) error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary)
	logPath := filepath.Join(t.TempDir(), "server.log")
	command.Env = append(os.Environ(), "RDL_MCP_LOG_LEVEL=DEBUG", "RDL_MCP_LOG_FILE="+logPath)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe() error = %v, want nil", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v, want nil", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	reader := bufio.NewReader(stdout)
	send := func(request string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, request); err != nil {
			t.Fatalf("send(%q) error = %v, want nil", request, err)
		}
	}
	receive := func() map[string]any {
		t.Helper()
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read response error = %v, want JSON-RPC line", err)
		}
		var message map[string]any
		if err := json.Unmarshal(line, &message); err != nil {
			t.Fatalf("Unmarshal(stdout line %q) error = %v, want JSON-RPC", line, err)
		}
		if message["jsonrpc"] != "2.0" {
			t.Errorf("stdout line = %#v, want JSON-RPC 2.0", message)
		}
		return message
	}
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"stdio-test","version":"1.0.0"}}}`)
	initial := receive()
	if initial["id"] != float64(1) || initial["result"] == nil {
		t.Errorf("initialize response = %#v, want id 1 result", initial)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listed := receive()
	if listed["id"] != float64(2) || listed["result"] == nil {
		t.Errorf("notification/list response = %#v, want only id 2 result", listed)
	}
	if malformed {
		send(`{invalid`)
		if line, err := reader.ReadBytes('\n'); err != io.EOF || len(line) != 0 {
			t.Errorf("invalid JSON stdout = %q, %v; want EOF without response", line, err)
		}
		if err := command.Wait(); err == nil {
			t.Error("rdl-mcp after invalid JSON = success, want decode failure")
		}
		if !strings.Contains(stderr.String(), "invalid character") {
			t.Errorf("stderr after invalid JSON = %q, want decode diagnostic", stderr.String())
		}
		return
	}
	send(`{"jsonrpc":"2.0","id":3,"method":"no_such_method"}`)
	unknown := receive()
	if unknown["id"] != float64(3) || unknown["error"] == nil {
		t.Errorf("unknown method response = %#v, want id 3 error", unknown)
	}
	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_rdl_report","arguments":{}}}`)
	missing := receive()
	if missing["id"] != float64(4) || missing["result"] == nil {
		t.Errorf("missing filepath response = %#v, want id 4 tool result", missing)
	} else if result, ok := missing["result"].(map[string]any); !ok || result["isError"] != true {
		t.Errorf("missing filepath response = %#v, want isError=true", missing)
	}
	send(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"describe_rdl_report","arguments":{"filepath":` + string(mustJSON(t, path)) + `}}}`)
	valid := receive()
	if valid["id"] != float64(5) || valid["result"] == nil {
		t.Errorf("valid call after errors = %#v, want id 5 result", valid)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("stdin.Close() error = %v, want nil", err)
	}
	remaining, readErr := io.ReadAll(reader)
	if err := command.Wait(); err != nil {
		t.Fatalf("rdl-mcp after EOF error = %v, stderr = %s", err, stderr.String())
	}
	if readErr != nil || len(remaining) != 0 {
		t.Errorf("stdout after EOF = %q, %v; want no extra output", remaining, readErr)
	}
	if logData, err := os.ReadFile(logPath); err != nil || len(logData) == 0 {
		t.Errorf("ReadFile(%q) = %q, %v; want diagnostic log", logPath, logData, err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(%v) error = %v, want nil", value, err)
	}
	return data
}
