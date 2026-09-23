package rdl

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// Validate returns the contract-compatible RDL validity result.
func Validate(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{"valid": false, "issues": []string{"Error: " + err.Error()}}
	}
	doc, err := ParseXML(data)
	if err != nil {
		return map[string]any{"valid": false, "issues": []string{"XML Parse Error: " + err.Error()}}
	}
	ns := doc.Root.URI
	issues := make([]string, 0)
	warnings := make([]string, 0)
	datasets := findAll(doc.Root, "DataSet", ns)
	if len(datasets) == 0 {
		return map[string]any{"valid": false, "issues": []string{"No datasets found"}}
	}
	datasetFields := make(map[string]map[string]struct{}, len(datasets))
	for _, dataset := range datasets {
		name := fmt.Sprint(attrOr(dataset, "Name", "Unknown"))
		fields := findAll(dataset, "Field", ns)
		if findChild(dataset, "Query", ns) == nil && len(fields) == 0 {
			issues = append(issues, fmt.Sprintf("Dataset %q has no Query element and no Fields", name))
		}
		datasetFields[name] = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			if name, ok := attrValue(field, "Name").(string); ok && name != "" {
				datasetFields[fmt.Sprint(attrOr(dataset, "Name", "Unknown"))][name] = struct{}{}
			}
		}
	}
	tablixes := findAll(doc.Root, "Tablix", ns)
	if len(tablixes) == 0 {
		issues = append(issues, "No Tablix (table) found")
	}
	for _, tablix := range tablixes {
		tablixName := fmt.Sprint(attrOr(tablix, "Name", "Unknown"))
		datasetName := textString(findChild(tablix, "DataSetName", ns))
		if datasetName == "" {
			warnings = append(warnings, fmt.Sprintf("Tablix %q has no DataSetName specified", tablixName))
			continue
		}
		if _, exists := datasetFields[datasetName]; !exists {
			issues = append(issues, fmt.Sprintf("Tablix %q references unknown dataset %q", tablixName, datasetName))
			continue
		}
		type invalidRef struct {
			field, dataset, location, errorText string
		}
		invalid := make([]invalidRef, 0)
		validateExpression := func(expression, location string) {
			if expression == "" {
				return
			}
			byDataset := ExtractFieldsWithContext(expression, datasetName)
			ordered := make([]string, 0, len(byDataset))
			if _, ok := byDataset[datasetName]; ok {
				ordered = append(ordered, datasetName)
			}
			for name := range byDataset {
				if name != datasetName {
					ordered = append(ordered, name)
				}
			}
			if len(ordered) > 1 {
				slices.Sort(ordered[1:])
			}
			for _, name := range ordered {
				available, known := datasetFields[name]
				for _, field := range byDataset[name] {
					if _, exists := available[field]; known && exists {
						continue
					}
					ref := invalidRef{field: field, dataset: name, location: location}
					if !known {
						ref.errorText = fmt.Sprintf("references unknown dataset %q", name)
					}
					invalid = append(invalid, ref)
				}
			}
		}
		processed := make(map[*Element]struct{})
		for _, group := range findAll(tablix, "GroupExpression", ns) {
			validateExpression(textString(group), "GroupExpression")
		}
		for _, sortExpression := range findAll(tablix, "SortExpression", ns) {
			if value := findChild(sortExpression, "Value", ns); value != nil {
				processed[value] = struct{}{}
				validateExpression(textString(value), "SortExpression")
			}
		}
		for _, value := range findAll(tablix, "Value", ns) {
			if _, seen := processed[value]; seen {
				continue
			}
			expression := textString(value)
			if !strings.HasPrefix(strings.TrimSpace(expression), "=") {
				continue
			}
			location := "unknown location"
			parent := value
			for range 10 {
				parent = parentOf(tablix, parent)
				if parent == nil {
					break
				}
				if parent.Name.Local == "Textbox" && parent.URI == ns {
					if name, ok := attrValue(parent, "Name").(string); ok && name != "" {
						location = name
					}
					break
				}
			}
			validateExpression(expression, location)
		}
		seen := make(map[string]struct{})
		for _, ref := range invalid {
			key := ref.dataset + "\x00" + ref.field
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if ref.errorText != "" {
				issues = append(issues, fmt.Sprintf("Tablix %q: Expression %s (field %q in %s)", tablixName, ref.errorText, ref.field, ref.location))
				continue
			}
			available := make([]string, 0, len(datasetFields[ref.dataset]))
			for name := range datasetFields[ref.dataset] {
				available = append(available, name)
			}
			slices.Sort(available)
			suffix := ""
			if len(available) > 10 {
				suffix = "..."
			}
			issues = append(issues, fmt.Sprintf("Tablix %q: Field %q not found in dataset %q (referenced in %s). Available fields: %s%s", tablixName, ref.field, ref.dataset, ref.location, strings.Join(available[:min(10, len(available))], ", "), suffix))
		}
	}
	result := map[string]any{"valid": true, "message": "RDL structure is valid"}
	if len(issues) > 0 {
		result = map[string]any{"valid": false, "issues": issues}
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}
	return result
}

func parentOf(root, target *Element) *Element {
	for _, child := range root.children {
		element, ok := child.(*Element)
		if !ok {
			continue
		}
		if element == target {
			return root
		}
		if parent := parentOf(element, target); parent != nil {
			return parent
		}
	}
	return nil
}
