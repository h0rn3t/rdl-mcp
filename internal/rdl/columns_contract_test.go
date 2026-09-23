package rdl

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestColumnToolsMatchStdioContract(t *testing.T) {
	base := filepath.Join("..", "..", "tests", "testdata", "rdl")
	data, err := os.ReadFile(filepath.Join(base, "stdio_contract.json"))
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
	cases := []struct {
		name string
		call func(string) (map[string]any, error)
	}{
		{"update_column_header", func(path string) (map[string]any, error) { return UpdateColumnHeader(path, "Name", "Full Name") }},
		{"update_column_header_missing", func(path string) (map[string]any, error) { return UpdateColumnHeader(path, "Missing", "New") }},
		{"update_column_width", func(path string) (map[string]any, error) { return UpdateColumnWidth(path, 0, "1.5in") }},
		{"update_column_width_bad_index", func(path string) (map[string]any, error) { return UpdateColumnWidth(path, 99, "1.5in") }},
		{"update_column_format", func(path string) (map[string]any, error) { return UpdateColumnFormat(path, 2, "C2") }},
		{"update_column_format_bad_index", func(path string) (map[string]any, error) { return UpdateColumnFormat(path, 99, "C2") }},
		{"update_column_format_negative", func(path string) (map[string]any, error) { return UpdateColumnFormat(path, -1, "C2") }},
		{"add_column", func(path string) (map[string]any, error) { return AddColumn(path, 1, "New", "Name", nil, nil, nil) }},
		{"add_column_custom", func(path string) (map[string]any, error) {
			return AddColumn(path, 1, "New", "Name", new("2cm"), new("C2"), nil)
		}},
		{"add_column_footer", func(path string) (map[string]any, error) {
			return AddColumn(path, 1, "New", "Name", nil, nil, new("=Sum(Fields!Amount.Value)"))
		}},
		{"add_column_end", func(path string) (map[string]any, error) { return AddColumn(path, -1, "Last", "Name", nil, nil, nil) }},
		{"add_column_bad_index", func(path string) (map[string]any, error) { return AddColumn(path, 99, "New", "Name", nil, nil, nil) }},
		{"remove_column", func(path string) (map[string]any, error) { return RemoveColumn(path, 1, true) }},
		{"remove_column_no_adjust", func(path string) (map[string]any, error) { return RemoveColumn(path, 1, false) }},
		{"remove_column_bad_index", func(path string) (map[string]any, error) { return RemoveColumn(path, 99, true) }},
		{"remove_column_negative", func(path string) (map[string]any, error) { return RemoveColumn(path, -1, true) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.rdl")
			source := "sample_report.xml"
			if tc.name == "add_column_footer" {
				source = "footer_report.xml"
			}
			original, err := os.ReadFile(filepath.Join(base, source))
			if err != nil {
				t.Fatalf("ReadFile(sample_report.xml) error = %v, want nil", err)
			}
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
			}
			got, err := tc.call(path)
			if err != nil {
				t.Fatalf("%s(%q) error = %v, want nil", tc.name, path, err)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal(%s) error = %v, want nil", tc.name, err)
			}
			var normalized any
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				t.Fatalf("Unmarshal(%s) error = %v, want nil", tc.name, err)
			}
			want := baseline.Cases[tc.name].Result.Content[0].Text
			if !reflect.DeepEqual(normalized, want) {
				t.Errorf("%s(%q) = %#v, want %#v", tc.name, path, normalized, want)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
			}
			xmlPath := filepath.Join(base, "xml", tc.name+".xml")
			wantXML, err := os.ReadFile(xmlPath)
			if os.IsNotExist(err) {
				if !bytes.Equal(after, original) {
					t.Errorf("%s(%q) changed RDL on a failed operation", tc.name, path)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", xmlPath, err)
			}
			if !reflect.DeepEqual(semanticXML(t, after), semanticXML(t, wantXML)) {
				t.Errorf("%s(%q) XML differs from recorded contract", tc.name, path)
			}
		})
	}
}

func semanticXML(t *testing.T, data []byte) []string {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(data))
	result := make([]string, 0)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatalf("Decoder.Token() error = %v, want nil", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			result = append(result, "start:"+value.Name.Space+"|"+value.Name.Local)
			attrs := make([]string, 0)
			for _, attr := range value.Attr {
				if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
					continue
				}
				attrs = append(attrs, attr.Name.Space+"|"+attr.Name.Local+"="+attr.Value)
			}
			slices.Sort(attrs)
			result = append(result, attrs...)
		case xml.EndElement:
			result = append(result, "end:"+value.Name.Space+"|"+value.Name.Local)
		case xml.CharData:
			if text := strings.TrimSpace(string(value)); text != "" {
				result = append(result, "text:"+text)
			}
		}
	}
}
