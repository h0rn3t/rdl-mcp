package rdl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDatasetAndParameterToolsMatchStdioContract(t *testing.T) {
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
		{"update_stored_procedure", "sample_report.xml", func(path string) (map[string]any, error) {
			return UpdateStoredProcedure(path, "MainDataset", "usp_NewProcedure")
		}},
		{"update_stored_procedure_missing_dataset", "sample_report.xml", func(path string) (map[string]any, error) {
			return UpdateStoredProcedure(path, "Missing", "usp_NewProcedure")
		}},
		{"add_dataset_field", "sample_report.xml", func(path string) (map[string]any, error) {
			return AddDatasetField(path, "MainDataset", "NewField", "NewField", "System.String")
		}},
		{"add_dataset_field_duplicate", "sample_report.xml", func(path string) (map[string]any, error) {
			return AddDatasetField(path, "MainDataset", "Name", "Name", "System.String")
		}},
		{"add_dataset_field_missing_dataset", "sample_report.xml", func(path string) (map[string]any, error) {
			return AddDatasetField(path, "Missing", "NewField", "NewField", "System.String")
		}},
		{"remove_dataset_field", "sample_report.xml", func(path string) (map[string]any, error) {
			return RemoveDatasetField(path, "MainDataset", "CreatedDate")
		}},
		{"remove_dataset_field_missing_field", "sample_report.xml", func(path string) (map[string]any, error) { return RemoveDatasetField(path, "MainDataset", "Missing") }},
		{"remove_dataset_field_missing_dataset", "sample_report.xml", func(path string) (map[string]any, error) { return RemoveDatasetField(path, "Missing", "Name") }},
		{"add_parameter", "sample_report.xml", func(path string) (map[string]any, error) {
			return AddParameter(path, "NewParam", "String", "New parameter")
		}},
		{"add_parameter_duplicate", "sample_report.xml", func(path string) (map[string]any, error) {
			return AddParameter(path, "StartDate", "DateTime", "Duplicate")
		}},
		{"add_parameter_no_section", "no_params.xml", func(path string) (map[string]any, error) {
			return AddParameter(path, "NewParam", "String", "New parameter")
		}},
		{"update_parameter", "sample_report.xml", func(path string) (map[string]any, error) {
			return UpdateParameter(path, "StartDate", new("Updated start"), nil)
		}},
		{"update_parameter_default", "sample_report.xml", func(path string) (map[string]any, error) {
			return UpdateParameter(path, "StartDate", nil, new("2026-01-01"))
		}},
		{"update_parameter_no_change", "sample_report.xml", func(path string) (map[string]any, error) { return UpdateParameter(path, "StartDate", nil, nil) }},
		{"update_parameter_missing", "sample_report.xml", func(path string) (map[string]any, error) { return UpdateParameter(path, "Missing", new("New"), nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.rdl")
			original, err := os.ReadFile(filepath.Join(base, tc.file))
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v, want nil", tc.file, err)
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
