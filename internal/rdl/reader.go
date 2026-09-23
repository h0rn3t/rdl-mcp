package rdl

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/dlclark/regexp2/v2"
)

const designerNamespace = "http://schemas.microsoft.com/SQLServer/reporting/reportdesigner"

// Describe returns the contract-compatible report summary.
func Describe(path string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	datasets := make([]map[string]any, 0)
	for _, dataset := range findAll(doc.Root, "DataSet", ns) {
		info := map[string]any{"name": attrValue(dataset, "Name"), "field_count": len(findAll(dataset, "Field", ns))}
		if query := findChild(dataset, "Query", ns); query != nil {
			info["command_type"] = textOr(findChild(query, "CommandType", ns), "Unknown")
			info["command"] = textOr(findChild(query, "CommandText", ns), "")
		} else {
			info["command_type"] = "Embedded"
			info["command"] = ""
		}
		datasets = append(datasets, info)
	}
	columns := 0
	tablixes := tablixSummaries(doc.Root, ns)
	if len(tablixes) > 0 {
		columns = tablixes[0]["column_count"].(int)
	}
	return map[string]any{
		"report_summary": map[string]any{"datasets": len(datasets), "parameters": len(findAll(doc.Root, "ReportParameter", ns)), "table_columns": columns},
		"datasets":       datasets,
		"tablixes":       tablixes,
		"filepath":       path,
	}, nil
}

// Datasets returns contract-compatible dataset details and field filtering.
func Datasets(path string, fieldLimit int, fieldPattern string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	var pattern *regexp2.Regexp
	if fieldPattern != "" {
		if translated, valid := translateFieldPattern(fieldPattern); valid {
			compiled, compileErr := regexp2.Compile(translated, regexp2.IgnoreCase)
			if compileErr == nil {
				pattern = compiled
			}
		}
	}
	ns := doc.Root.URI
	datasets := make([]map[string]any, 0)
	for _, dataset := range findAll(doc.Root, "DataSet", ns) {
		info := map[string]any{
			"name":             attrValue(dataset, "Name"),
			"datasource":       "",
			"command_type":     "Embedded",
			"command_text":     "",
			"query_parameters": make([]map[string]any, 0),
		}
		if query := findChild(dataset, "Query", ns); query != nil {
			info["datasource"] = textOr(findChild(query, "DataSourceName", ns), "")
			info["command_type"] = textOr(findChild(query, "CommandType", ns), "Unknown")
			info["command_text"] = textOr(findChild(query, "CommandText", ns), "")
			params := make([]map[string]any, 0)
			for _, param := range findAll(query, "QueryParameter", ns) {
				params = append(params, map[string]any{"name": attrValue(param, "Name"), "value": textOr(findChild(param, "Value", ns), "")})
			}
			info["query_parameters"] = params
		}
		allFields := make([]map[string]any, 0)
		filtered := make([]map[string]any, 0)
		for _, field := range findAll(dataset, "Field", ns) {
			fieldName := attrValue(field, "Name")
			item := map[string]any{
				"name":       fieldName,
				"data_field": textOr(findChild(field, "DataField", ns), ""),
				"type":       textOr(findFirst(field, "TypeName", designerNamespace), "Unknown"),
			}
			allFields = append(allFields, item)
			if pattern != nil {
				matched, err := pattern.MatchString(fmt.Sprint(fieldName))
				if err != nil {
					return nil, fmt.Errorf("match field pattern: %w", err)
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, item)
		}
		info["field_count"] = len(allFields)
		if fieldLimit != 0 {
			if fieldLimit == -1 {
				info["fields"] = filtered
				info["fields_truncated"] = false
			} else {
				end := fieldLimit
				if end < 0 {
					end = max(0, len(filtered)+end)
				}
				end = min(end, len(filtered))
				info["fields"] = filtered[:end]
				info["fields_truncated"] = len(filtered) > fieldLimit
			}
		}
		datasets = append(datasets, info)
	}
	return map[string]any{"datasets": datasets}, nil
}

func translateFieldPattern(pattern string) (string, bool) {
	var output strings.Builder
	inClass := false
	for i := 0; i < len(pattern); {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			if pattern[i+1] == 'k' {
				return "", false
			}
			output.WriteString(pattern[i : i+2])
			i += 2
			continue
		}
		switch pattern[i] {
		case '[':
			inClass = true
		case ']':
			inClass = false
		}
		if !inClass && strings.HasPrefix(pattern[i:], "(?P<") {
			output.WriteString("(?<")
			i += 4
			continue
		}
		if !inClass && strings.HasPrefix(pattern[i:], "(?<") && !strings.HasPrefix(pattern[i:], "(?<=") && !strings.HasPrefix(pattern[i:], "(?<!") {
			return "", false
		}
		if !inClass && strings.HasPrefix(pattern[i:], "(?'") {
			return "", false
		}
		if !inClass && strings.HasPrefix(pattern[i:], "(?P=") {
			if end := strings.IndexByte(pattern[i+4:], ')'); end >= 0 {
				output.WriteString(`\k<`)
				output.WriteString(pattern[i+4 : i+4+end])
				output.WriteByte('>')
				i += 5 + end
				continue
			}
		}
		output.WriteByte(pattern[i])
		i++
	}
	return output.String(), true
}

// Parameters returns contract-compatible report parameter details.
func Parameters(path string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	parameters := make([]map[string]any, 0)
	for _, param := range findAll(doc.Root, "ReportParameter", ns) {
		info := map[string]any{
			"name":      attrValue(param, "Name"),
			"data_type": textOr(findChild(param, "DataType", ns), "Unknown"),
			"prompt":    textOr(findChild(param, "Prompt", ns), ""),
		}
		if defaults := findChild(param, "DefaultValue", ns); defaults != nil {
			if values := findChild(defaults, "Values", ns); values != nil {
				items := make([]any, 0)
				for _, value := range findChildren(values, "Value", ns) {
					if text := textValue(value); text != nil && text != "" {
						items = append(items, text)
					}
				}
				if len(findChildren(values, "Value", ns)) > 0 {
					info["default_values"] = items
				}
			}
		}
		if valid := findChild(param, "ValidValues", ns); valid != nil {
			if ref := findChild(valid, "DataSetReference", ns); ref != nil {
				info["valid_values_dataset"] = textOr(findChild(ref, "DataSetName", ns), "")
			}
			if list := findChild(valid, "ParameterValues", ns); list != nil {
				values := findChildren(list, "ParameterValue", ns)
				if len(values) > 0 {
					items := make([]map[string]any, 0, len(values))
					for _, value := range values {
						itemValue := textOr(findChild(value, "Value", ns), "")
						items = append(items, map[string]any{"value": itemValue, "label": textOr(findChild(value, "Label", ns), itemValue)})
					}
					info["valid_values"] = items
				}
			}
		}
		parameters = append(parameters, info)
	}
	return map[string]any{"parameters": parameters}, nil
}

// Columns returns contract-compatible Tablix column details.
func Columns(path, tablixName string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablix, names := selectTablix(doc.Root, ns, tablixName)
	if tablix == nil {
		message := "No Tablix found"
		if len(names) > 1 && tablixName == "" {
			message = "Multiple Tablix found; specify tablix_name"
		} else if tablixName != "" {
			message = fmt.Sprintf("Tablix %q not found", tablixName)
		}
		result := map[string]any{"columns": make([]map[string]any, 0), "error": message}
		if len(names) > 0 {
			result["tablixes"] = names
		}
		return result, nil
	}
	return map[string]any{"columns": columnsForTablix(tablix, ns)}, nil
}

func tablixSummaries(root *Element, ns string) []map[string]any {
	tablixes := make([]map[string]any, 0)
	for _, tablix := range findAll(root, "Tablix", ns) {
		name, _ := attrValue(tablix, "Name").(string)
		tablixes = append(tablixes, map[string]any{
			"name":         name,
			"dataset_name": textString(findChild(tablix, "DataSetName", ns)),
			"column_count": len(columnsForTablix(tablix, ns)),
		})
	}
	return tablixes
}

func selectTablix(root *Element, ns, tablixName string) (*Element, []string) {
	tablixes := findAll(root, "Tablix", ns)
	names := make([]string, 0, len(tablixes))
	for _, tablix := range tablixes {
		name, _ := attrValue(tablix, "Name").(string)
		names = append(names, name)
		if tablixName != "" && name == tablixName {
			return tablix, names
		}
	}
	if tablixName == "" && len(tablixes) == 1 {
		return tablixes[0], names
	}
	return nil, names
}

func columnsForTablix(tablix *Element, ns string) []map[string]any {
	widths := make([]any, 0)
	for _, group := range findAll(tablix, "TablixColumns", ns) {
		for _, column := range findChildren(group, "TablixColumn", ns) {
			widths = append(widths, textOr(findChild(column, "Width", ns), ""))
		}
	}
	var headerRow, dataRow *Element
	for _, body := range findAll(tablix, "TablixBody", ns) {
		for _, rows := range findChildren(body, "TablixRows", ns) {
			for _, row := range findChildren(rows, "TablixRow", ns) {
				cells := tablixCells(row, ns)
				switch detectRowType(cells, ns) {
				case "header":
					if headerRow == nil {
						headerRow = row
					}
				case "data":
					if dataRow == nil {
						dataRow = row
					}
				}
			}
		}
	}
	columns := make([]map[string]any, 0)
	if headerRow != nil {
		for index, cell := range tablixCells(headerRow, ns) {
			header, textboxName := "", ""
			if textbox := findFirst(cell, "Textbox", ns); textbox != nil {
				textboxName = fmt.Sprint(attrOr(textbox, "Name", ""))
				if value := textString(findFirst(textbox, "Value", ns)); value != "" {
					header = value
					if strings.HasPrefix(header, "=") {
						if _, field, ok := strings.Cut(header, "Fields!"); ok {
							field, _, _ = strings.Cut(field, ".")
							field, _, _ = strings.Cut(field, ")")
							header = field
						}
					}
				}
			}
			width := any("")
			if index < len(widths) {
				width = widths[index]
			}
			columns = append(columns, map[string]any{"index": index, "header": header, "width": width, "textbox_name": textboxName})
		}
	}
	if dataRow != nil {
		for index, cell := range tablixCells(dataRow, ns) {
			if index >= len(columns) {
				break
			}
			textbox := findFirst(cell, "Textbox", ns)
			if textbox == nil {
				continue
			}
			if binding := textString(findFirst(textbox, "Value", ns)); binding != "" {
				columns[index]["field_binding"] = binding
				if strings.HasPrefix(binding, "=") {
					if _, field, ok := strings.Cut(binding, "Fields!"); ok {
						field, _, _ = strings.Cut(field, ".")
						columns[index]["field_name"] = field
					}
				}
			}
			if format := textString(findFirst(textbox, "Format", ns)); format != "" {
				columns[index]["format"] = format
			}
		}
	}
	return columns
}

func findChildren(parent *Element, local, uri string) []*Element {
	children := make([]*Element, 0)
	if parent == nil {
		return children
	}
	for _, child := range parent.children {
		if element, ok := child.(*Element); ok && element.Name.Local == local && element.URI == uri {
			children = append(children, element)
		}
	}
	return children
}

func findChild(parent *Element, local, uri string) *Element {
	for _, child := range findChildren(parent, local, uri) {
		return child
	}
	return nil
}

func findAll(parent *Element, local, uri string) []*Element {
	result := make([]*Element, 0)
	if parent == nil {
		return result
	}
	for _, child := range parent.children {
		element, ok := child.(*Element)
		if !ok {
			continue
		}
		if element.Name.Local == local && element.URI == uri {
			result = append(result, element)
		}
		result = append(result, findAll(element, local, uri)...)
	}
	return result
}

func findFirst(parent *Element, local, uri string) *Element {
	for _, child := range findAll(parent, local, uri) {
		return child
	}
	return nil
}

func attrValue(element *Element, name string) any {
	for _, attr := range element.Attrs {
		if attr.Name == (xml.Name{Local: name}) {
			return attr.Value
		}
	}
	return nil
}

func attrOr(element *Element, name string, fallback any) any {
	if value := attrValue(element, name); value != nil {
		return value
	}
	return fallback
}

func textValue(element *Element) any {
	if element == nil || len(element.children) == 0 {
		return nil
	}
	if value, ok := element.children[0].(xml.CharData); ok {
		return string(value)
	}
	return nil
}

func textOr(element *Element, fallback any) any {
	if element == nil {
		return fallback
	}
	return textValue(element)
}

func textString(element *Element) string {
	if value, ok := textValue(element).(string); ok {
		return value
	}
	return ""
}

func tablixCells(row *Element, ns string) []*Element {
	return findChildren(findChild(row, "TablixCells", ns), "TablixCell", ns)
}

func detectRowType(cells []*Element, ns string) string {
	static, data, aggregate := 0, 0, 0
	for _, cell := range cells {
		value := strings.TrimSpace(textString(findFirst(findFirst(cell, "Textbox", ns), "Value", ns)))
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, "=") {
			static++
			continue
		}
		if strings.Contains(value, "Sum(") || strings.Contains(value, "Count(") || strings.Contains(value, "Avg(") || strings.Contains(value, "Min(") || strings.Contains(value, "Max(") || strings.Contains(value, "First(") {
			aggregate++
		} else {
			data++
		}
	}
	if static+data+aggregate == 0 {
		return "empty"
	}
	if aggregate > 0 && aggregate >= data {
		return "footer"
	}
	if static > data {
		return "header"
	}
	return "data"
}
