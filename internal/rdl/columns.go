package rdl

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// UpdateColumnHeader changes the first matching Value element.
func UpdateColumnHeader(path, oldHeader, newHeader string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	for _, value := range findAll(doc.Root, "Value", doc.Root.URI) {
		if textString(value) != oldHeader {
			continue
		}
		if err := value.SetText(newHeader); err != nil {
			return nil, err
		}
		if err := doc.SaveXMLFile(path); err != nil {
			return nil, err
		}
		return map[string]any{"success": true, "message": fmt.Sprintf("Updated header from %q to %q", oldHeader, newHeader)}, nil
	}
	return map[string]any{"success": false, "message": fmt.Sprintf("Header %q not found", oldHeader)}, nil
}

// UpdateColumnWidth changes a Tablix column width.
func UpdateColumnWidth(path string, index int, width string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablix := findFirst(doc.Root, "Tablix", ns)
	if tablix == nil {
		return map[string]any{"success": false, "message": "No Tablix found"}, nil
	}
	columns := tablixColumns(tablix, ns)
	if index < 0 || index >= len(columns) {
		return map[string]any{"success": false, "message": fmt.Sprintf("Invalid column index %d", index)}, nil
	}
	value := findChild(columns[index], "Width", ns)
	if value == nil {
		value = appendElement(columns[index], "Width")
	}
	if err := value.SetText(width); err != nil {
		return nil, err
	}
	updateTablixWidth(tablix, ns)
	if err := doc.SaveXMLFile(path); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "message": fmt.Sprintf("Updated column %d width to %s", index, width)}, nil
}

// UpdateColumnFormat changes the format of a data row TextRun.
func UpdateColumnFormat(path string, index int, format string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablix := findFirst(doc.Root, "Tablix", ns)
	if tablix == nil {
		return map[string]any{"success": false, "message": "No Tablix found in report"}, nil
	}
	for _, row := range tablixRows(tablix, ns) {
		cells := tablixCells(row, ns)
		if detectRowType(cells, ns) != "data" {
			continue
		}
		if index >= len(cells) {
			return map[string]any{"success": false, "message": fmt.Sprintf("Column index %d out of range", index)}, nil
		}
		actual := index
		if actual < 0 {
			actual += len(cells)
		}
		if actual < 0 {
			return nil, fmt.Errorf("column index %d out of range", index)
		}
		textbox := findFirst(cells[actual], "Textbox", ns)
		if textbox == nil {
			return map[string]any{"success": false, "message": fmt.Sprintf("No textbox found in column %d", index)}, nil
		}
		textRun := findFirst(textbox, "TextRun", ns)
		if textRun == nil {
			return map[string]any{"success": false, "message": "No TextRun found in textbox"}, nil
		}
		style := findChild(textRun, "Style", ns)
		if style == nil {
			style = appendElement(textRun, "Style")
		}
		value := findChild(style, "Format", ns)
		if value == nil {
			value = appendElement(style, "Format")
		}
		if err := value.SetText(format); err != nil {
			return nil, err
		}
		if err := doc.SaveXMLFile(path); err != nil {
			return nil, err
		}
		return map[string]any{"success": true, "message": fmt.Sprintf("Updated format for column %d to %q", index, format)}, nil
	}
	return map[string]any{"success": false, "message": "No data row found"}, nil
}

// AddColumn inserts a Tablix column and matching row cells.
func AddColumn(path string, index int, header, binding string, width, format, footer *string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablix := findFirst(doc.Root, "Tablix", ns)
	if tablix == nil {
		return map[string]any{"success": false, "message": "No Tablix found in report"}, nil
	}
	container := tablixColumnContainer(tablix, ns)
	if container == nil {
		return map[string]any{"success": false, "message": "No TablixColumns found"}, nil
	}
	count := len(findChildren(container, "TablixColumn", ns))
	if index == -1 {
		index = count
	}
	if index < 0 || index > count {
		return map[string]any{"success": false, "message": fmt.Sprintf("Invalid column index %d. Must be 0-%d", index, count)}, nil
	}
	column := &Element{Name: xml.Name{Space: container.Name.Space, Local: "TablixColumn"}, URI: ns}
	columnWidth := "1in"
	if width != nil {
		columnWidth = *width
	}
	if err := appendElement(column, "Width").SetText(columnWidth); err != nil {
		return nil, err
	}
	insertElement(container, "TablixColumn", ns, index, column)
	for _, hierarchy := range findAll(tablix, "TablixColumnHierarchy", ns) {
		if members := findChild(hierarchy, "TablixMembers", ns); members != nil {
			insertElement(members, "TablixMember", ns, index, &Element{Name: xml.Name{Space: members.Name.Space, Local: "TablixMember"}, URI: ns})
			break
		}
	}
	for rowIndex, row := range tablixRows(tablix, ns) {
		cells := findChild(row, "TablixCells", ns)
		if cells == nil {
			continue
		}
		cell := newTableCell(cells.Name.Space, ns, detectRowType(tablixCells(row, ns), ns), rowIndex, index, header, binding, format, footer)
		insertElement(cells, "TablixCell", ns, index, cell)
	}
	updateTablixWidth(tablix, ns)
	if err := doc.SaveXMLFile(path); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "message": fmt.Sprintf("Added column %q at position %d", header, index)}, nil
}

// RemoveColumn deletes a Tablix column and optionally adjusts the page width.
func RemoveColumn(path string, index int, autoAdjust bool) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablix := findFirst(doc.Root, "Tablix", ns)
	if tablix == nil {
		return map[string]any{"success": false, "message": "No Tablix found in report"}, nil
	}
	container := tablixColumnContainer(tablix, ns)
	if container == nil {
		return map[string]any{"success": false, "message": "No TablixColumns found"}, nil
	}
	columns := findChildren(container, "TablixColumn", ns)
	if index < 0 || index >= len(columns) {
		return map[string]any{"success": false, "message": fmt.Sprintf("Invalid column index %d", index)}, nil
	}
	removeElement(container, columns[index])
	for _, hierarchy := range findAll(tablix, "TablixColumnHierarchy", ns) {
		if members := findChild(hierarchy, "TablixMembers", ns); members != nil {
			items := findChildren(members, "TablixMember", ns)
			if index < len(items) {
				removeElement(members, items[index])
			}
			break
		}
	}
	for _, row := range tablixRows(tablix, ns) {
		if cells := findChild(row, "TablixCells", ns); cells != nil {
			items := findChildren(cells, "TablixCell", ns)
			if index < len(items) {
				removeElement(cells, items[index])
			}
		}
	}
	width := updateTablixWidth(tablix, ns)
	if autoAdjust {
		updatePageWidth(doc.Root, ns, width)
	}
	if err := doc.SaveXMLFile(path); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "message": fmt.Sprintf("Removed column at index %d", index)}, nil
}

func tablixColumnContainer(tablix *Element, ns string) *Element {
	for _, body := range findAll(tablix, "TablixBody", ns) {
		if columns := findChild(body, "TablixColumns", ns); columns != nil {
			return columns
		}
	}
	return nil
}

func tablixColumns(tablix *Element, ns string) []*Element {
	columns := make([]*Element, 0)
	for _, body := range findAll(tablix, "TablixBody", ns) {
		for _, group := range findChildren(body, "TablixColumns", ns) {
			columns = append(columns, findChildren(group, "TablixColumn", ns)...)
		}
	}
	return columns
}

func tablixRows(tablix *Element, ns string) []*Element {
	rows := make([]*Element, 0)
	for _, body := range findAll(tablix, "TablixBody", ns) {
		for _, group := range findChildren(body, "TablixRows", ns) {
			rows = append(rows, findChildren(group, "TablixRow", ns)...)
		}
	}
	return rows
}

func appendElement(parent *Element, local string) *Element {
	child := &Element{Name: xml.Name{Space: parent.Name.Space, Local: local}, URI: parent.URI}
	parent.children = append(parent.children, child)
	return child
}

func insertElement(parent *Element, local, uri string, index int, child *Element) {
	position := len(parent.children)
	seen := 0
	for i, item := range parent.children {
		element, ok := item.(*Element)
		if !ok || element.Name.Local != local || element.URI != uri {
			continue
		}
		if seen == index {
			position = i
			break
		}
		seen++
	}
	parent.children = slices.Insert(parent.children, position, any(child))
}

func removeElement(parent, target *Element) {
	for i, item := range parent.children {
		if item == target {
			parent.children = slices.Delete(parent.children, i, i+1)
			return
		}
	}
}

func newTableCell(prefix, uri, rowType string, rowIndex, columnIndex int, header, binding string, format, footer *string) *Element {
	cell := &Element{Name: xml.Name{Space: prefix, Local: "TablixCell"}, URI: uri}
	contents := appendElement(cell, "CellContents")
	textbox := appendElement(contents, "Textbox")
	textbox.Attrs = []xml.Attr{{Name: xml.Name{Local: "Name"}, Value: fmt.Sprintf("Textbox_r%d_c%d", rowIndex, columnIndex)}}
	textRun := appendElement(appendElement(appendElement(appendElement(textbox, "Paragraphs"), "Paragraph"), "TextRuns"), "TextRun")
	value := appendElement(textRun, "Value")
	switch rowType {
	case "header":
		_ = value.SetText(header)
	case "data":
		_ = value.SetText(binding)
		if format != nil && *format != "" {
			_ = appendElement(appendElement(textRun, "Style"), "Format").SetText(*format)
		}
	case "footer":
		if footer != nil {
			_ = value.SetText(*footer)
		} else {
			_ = value.SetText("")
		}
	}
	return cell
}

func parseDimension(value string) float64 {
	if value == "" {
		return 0
	}
	divisor := 1.0
	for _, unit := range []struct {
		suffix  string
		divisor float64
	}{{"in", 1}, {"cm", 2.54}, {"mm", 25.4}, {"pt", 72}} {
		if strings.HasSuffix(value, unit.suffix) {
			divisor = unit.divisor
			value = strings.TrimSuffix(value, unit.suffix)
			break
		}
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return number / divisor
}

func updateTablixWidth(tablix *Element, ns string) float64 {
	total := 0.0
	for _, column := range tablixColumns(tablix, ns) {
		total += parseDimension(textString(findChild(column, "Width", ns)))
	}
	if width := findChild(tablix, "Width", ns); width != nil {
		_ = width.SetText(fmt.Sprintf("%.2fin", total))
	}
	return total
}

func updatePageWidth(root *Element, ns string, tablixWidth float64) {
	page := findFirst(root, "Page", ns)
	if page == nil {
		return
	}
	left := parseDimension(textString(findChild(page, "LeftMargin", ns)))
	right := parseDimension(textString(findChild(page, "RightMargin", ns)))
	if width := findChild(page, "PageWidth", ns); width != nil {
		_ = width.SetText(fmt.Sprintf("%.2fin", tablixWidth+left+right))
	}
}
