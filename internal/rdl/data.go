package rdl

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
)

// UpdateStoredProcedure changes the CommandText of a named dataset.
func UpdateStoredProcedure(path, datasetName, procedure string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	for _, dataset := range findAll(doc.Root, "DataSet", ns) {
		if attrValue(dataset, "Name") != datasetName {
			continue
		}
		if query := findChild(dataset, "Query", ns); query != nil {
			if command := findChild(query, "CommandText", ns); command != nil {
				if err := command.SetText(procedure); err != nil {
					return nil, err
				}
				if err := doc.SaveXMLFile(path); err != nil {
					return nil, err
				}
				return map[string]any{"success": true, "message": fmt.Sprintf("Updated stored procedure to %q", procedure)}, nil
			}
		}
	}
	return map[string]any{"success": false, "message": fmt.Sprintf("Dataset %q not found", datasetName)}, nil
}

// AddDatasetField adds a field to a named dataset.
func AddDatasetField(path, datasetName, fieldName, dataField, typeName string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	for _, dataset := range findAll(doc.Root, "DataSet", ns) {
		if attrValue(dataset, "Name") != datasetName {
			continue
		}
		fields := findChild(dataset, "Fields", ns)
		if fields == nil {
			fields = &Element{Name: xml.Name{Space: dataset.Name.Space, Local: "Fields"}, URI: ns}
			insertAfter(dataset, findChild(dataset, "Query", ns), fields)
		}
		for _, field := range findChildren(fields, "Field", ns) {
			if attrValue(field, "Name") == fieldName {
				return map[string]any{"success": false, "message": fmt.Sprintf("Field %q already exists", fieldName)}, nil
			}
		}
		field := appendElement(fields, "Field")
		field.Attrs = []xml.Attr{{Name: xml.Name{Local: "Name"}, Value: fieldName}}
		if err := appendElement(field, "DataField").SetText(dataField); err != nil {
			return nil, err
		}
		prefix := ensureDesignerPrefix(doc.Root)
		kind := &Element{Name: xml.Name{Space: prefix, Local: "TypeName"}, URI: designerNamespace}
		field.children = append(field.children, kind)
		if err := kind.SetText(typeName); err != nil {
			return nil, err
		}
		if err := doc.SaveXMLFile(path); err != nil {
			return nil, err
		}
		return map[string]any{"success": true, "message": fmt.Sprintf("Added field %q to dataset %q", fieldName, datasetName)}, nil
	}
	return map[string]any{"success": false, "message": fmt.Sprintf("Dataset %q not found", datasetName)}, nil
}

// RemoveDatasetField removes a field from a named dataset.
func RemoveDatasetField(path, datasetName, fieldName string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	for _, dataset := range findAll(doc.Root, "DataSet", ns) {
		if attrValue(dataset, "Name") != datasetName {
			continue
		}
		fields := findChild(dataset, "Fields", ns)
		if fields == nil {
			return map[string]any{"success": false, "message": fmt.Sprintf("Dataset %q has no fields", datasetName)}, nil
		}
		for _, field := range findChildren(fields, "Field", ns) {
			if attrValue(field, "Name") != fieldName {
				continue
			}
			removeElement(fields, field)
			if err := doc.SaveXMLFile(path); err != nil {
				return nil, err
			}
			return map[string]any{"success": true, "message": fmt.Sprintf("Removed field %q from dataset %q", fieldName, datasetName)}, nil
		}
		return map[string]any{"success": false, "message": fmt.Sprintf("Field %q not found in dataset %q", fieldName, datasetName)}, nil
	}
	return map[string]any{"success": false, "message": fmt.Sprintf("Dataset %q not found", datasetName)}, nil
}

// AddParameter adds a report parameter, creating its section when absent.
func AddParameter(path, name, dataType, prompt string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	section := findChild(doc.Root, "ReportParameters", ns)
	if section == nil {
		section = &Element{Name: xml.Name{Space: doc.Root.Name.Space, Local: "ReportParameters"}, URI: ns}
		insertAfter(doc.Root, findChild(doc.Root, "DataSets", ns), section)
	}
	for _, param := range findChildren(section, "ReportParameter", ns) {
		if attrValue(param, "Name") == name {
			return map[string]any{"success": false, "message": fmt.Sprintf("Parameter %q already exists", name)}, nil
		}
	}
	param := appendElement(section, "ReportParameter")
	param.Attrs = []xml.Attr{{Name: xml.Name{Local: "Name"}, Value: name}}
	if err := appendElement(param, "DataType").SetText(dataType); err != nil {
		return nil, err
	}
	if err := appendElement(param, "Prompt").SetText(prompt); err != nil {
		return nil, err
	}
	if err := doc.SaveXMLFile(path); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "message": fmt.Sprintf("Added parameter %q", name)}, nil
}

// UpdateParameter changes a report parameter's prompt or default value.
func UpdateParameter(path, name string, prompt, defaultValue *string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	for _, param := range findAll(doc.Root, "ReportParameter", ns) {
		if attrValue(param, "Name") != name {
			continue
		}
		changes := make([]string, 0, 2)
		if prompt != nil {
			value := findChild(param, "Prompt", ns)
			if value == nil {
				value = appendElement(param, "Prompt")
			}
			if err := value.SetText(*prompt); err != nil {
				return nil, err
			}
			changes = append(changes, fmt.Sprintf("prompt to %q", *prompt))
		}
		if defaultValue != nil {
			defaults := findChild(param, "DefaultValue", ns)
			if defaults == nil {
				defaults = appendElement(param, "DefaultValue")
			}
			values := findChild(defaults, "Values", ns)
			if values == nil {
				values = appendElement(defaults, "Values")
			}
			value := findChild(values, "Value", ns)
			if value == nil {
				value = appendElement(values, "Value")
			}
			if err := value.SetText(*defaultValue); err != nil {
				return nil, err
			}
			changes = append(changes, fmt.Sprintf("default value to %q", *defaultValue))
		}
		if len(changes) == 0 {
			return map[string]any{"success": false, "message": "No changes specified"}, nil
		}
		if err := doc.SaveXMLFile(path); err != nil {
			return nil, err
		}
		return map[string]any{"success": true, "message": fmt.Sprintf("Updated parameter %q: %s", name, strings.Join(changes, ", "))}, nil
	}
	return map[string]any{"success": false, "message": fmt.Sprintf("Parameter %q not found", name)}, nil
}

func insertAfter(parent, after, child *Element) {
	if after == nil {
		for i, item := range parent.children {
			if _, ok := item.(*Element); ok {
				parent.children = slices.Insert(parent.children, i, any(child))
				return
			}
		}
		parent.children = append(parent.children, child)
		return
	}
	for i, item := range parent.children {
		if item == after {
			parent.children = slices.Insert(parent.children, i+1, any(child))
			return
		}
	}
}

func ensureDesignerPrefix(root *Element) string {
	for _, attr := range root.Attrs {
		if attr.Name.Space == "xmlns" && attr.Value == designerNamespace {
			return attr.Name.Local
		}
	}
	for index := 0; ; index++ {
		prefix := "rd"
		if index > 0 {
			prefix = fmt.Sprintf("rd%d", index)
		}
		used := false
		for _, attr := range root.Attrs {
			if attr.Name == (xml.Name{Space: "xmlns", Local: prefix}) {
				used = true
				break
			}
		}
		if !used {
			root.Attrs = append(root.Attrs, xml.Attr{Name: xml.Name{Space: "xmlns", Local: prefix}, Value: designerNamespace})
			return prefix
		}
	}
}
