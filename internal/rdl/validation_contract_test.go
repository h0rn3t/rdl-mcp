package rdl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateMatchesStdioContract(t *testing.T) {
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
	for _, tc := range []struct{ name, file string }{
		{"validate_rdl", "sample_report.xml"},
		{"validate_rdl_empty", "empty_report.xml"},
		{"validate_rdl_rich", "rich_report.xml"},
		{"validate_rdl_bad_field", "bad_field.xml"},
		{"validate_rdl_malformed", "malformed_report.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(base, tc.file)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
			}
			got := Validate(path)
			want := baseline.Cases[tc.name].Result.Content[0].Text
			if tc.name == "validate_rdl_malformed" {
				issues, ok := got["issues"].([]string)
				if got["valid"] != false || !ok || len(issues) != 1 || !strings.HasPrefix(issues[0], "XML Parse Error:") {
					t.Errorf("Validate(%q) = %#v, want invalid XML classification", path, got)
				}
			} else {
				encoded, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("Marshal(Validate(%q)) error = %v, want nil", path, err)
				}
				var normalized any
				if err := json.Unmarshal(encoded, &normalized); err != nil {
					t.Fatalf("Unmarshal(Validate(%q)) error = %v, want nil", path, err)
				}
				if !reflect.DeepEqual(normalized, want) {
					t.Errorf("Validate(%q) = %#v, want %#v", path, normalized, want)
				}
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
			}
			if string(before) != string(after) {
				t.Errorf("Validate(%q) changed the RDL file", path)
			}
		})
	}
}

func TestExtractFieldsMatchContract(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "rdl", "field_reference_contract.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	var cases map[string]struct {
		Expression     string              `json:"expression"`
		DefaultDataset string              `json:"default_dataset"`
		WithContext    map[string][]string `json:"with_context"`
		Fields         []string            `json:"fields"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v, want nil", path, err)
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ExtractFieldsWithContext(tc.Expression, tc.DefaultDataset); !reflect.DeepEqual(got, tc.WithContext) {
				t.Errorf("ExtractFieldsWithContext(%q, %q) = %#v, want %#v", tc.Expression, tc.DefaultDataset, got, tc.WithContext)
			}
			if got := ExtractFields(tc.Expression); !reflect.DeepEqual(got, tc.Fields) {
				t.Errorf("ExtractFields(%q) = %#v, want %#v", tc.Expression, got, tc.Fields)
			}
		})
	}
}
