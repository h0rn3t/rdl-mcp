package rdl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadToolsMatchStdioContract(t *testing.T) {
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
		file string
		call func(string) (map[string]any, error)
	}{
		{"describe_rdl_report", "sample_report.xml", Describe},
		{"get_rdl_datasets", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, 0, "") }},
		{"get_rdl_datasets_all", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "") }},
		{"get_rdl_datasets_filtered", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "^Name$") }},
		{"get_rdl_datasets_lookbehind", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "(?<=N)ame") }},
		{"get_rdl_datasets_named_group", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "(?P<initial>N)ame") }},
		{"get_rdl_datasets_named_backreference", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "(?P<letter>o)(?P=letter)") }},
		{"get_rdl_datasets_backreference", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, `(o)\1`) }},
		{"get_rdl_datasets_invalid_pattern", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "[") }},
		{"get_rdl_datasets_dotnet_pattern", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "(?<named>N)ame") }},
		{"get_rdl_datasets_negative_limit", "sample_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -2, "") }},
		{"get_rdl_parameters", "sample_report.xml", Parameters},
		{"get_rdl_columns", "sample_report.xml", func(path string) (map[string]any, error) { return Columns(path, "") }},
		{"describe_rdl_report_empty", "empty_report.xml", Describe},
		{"get_rdl_datasets_empty", "empty_report.xml", func(path string) (map[string]any, error) { return Datasets(path, 0, "") }},
		{"get_rdl_parameters_empty", "empty_report.xml", Parameters},
		{"get_rdl_columns_empty", "empty_report.xml", func(path string) (map[string]any, error) { return Columns(path, "") }},
		{"describe_rdl_report_rich", "rich_report.xml", Describe},
		{"get_rdl_datasets_rich", "rich_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, "") }},
		{"get_rdl_datasets_unicode", "rich_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, `^\w+$`) }},
		{"get_rdl_datasets_named_unicode", "rich_report.xml", func(path string) (map[string]any, error) { return Datasets(path, -1, `^(?P<word>\w+)$`) }},
		{"get_rdl_parameters_rich", "rich_report.xml", Parameters},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(base, tc.file)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
			}
			got, err := tc.call(path)
			if err != nil {
				t.Fatalf("%s(%q) error = %v, want nil", tc.name, path, err)
			}
			if _, ok := got["filepath"]; ok {
				got["filepath"] = "<RDL>"
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
			if string(after) != string(before) {
				t.Errorf("%s(%q) changed the RDL file", tc.name, path)
			}
		})
	}
}
