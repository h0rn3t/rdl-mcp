package rdl

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// TextboxPatch describes one guarded static-text replacement.
type TextboxPatch struct {
	TextboxName    string `json:"textbox_name"`
	ExpectedValue  string `json:"expected_value"`
	NewValue       string `json:"new_value"`
	ParagraphIndex *int   `json:"paragraph_index,omitempty"`
	TextRunIndex   *int   `json:"text_run_index,omitempty"`
}

type textboxValue struct {
	element        *Element
	value          *Element
	paragraphIndex int
	textRunIndex   int
}

// Textboxes returns the report's Textbox values, locations, geometry, and core styles.
func Textboxes(path string) (map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"filepath": path, "textboxes": textboxesFromDocument(doc.Root, doc.Root.URI)}, nil
}

func textboxesFromDocument(root *Element, ns string) []map[string]any {
	textboxes := make([]map[string]any, 0)
	for _, textbox := range findAll(root, "Textbox", ns) {
		name, _ := attrValue(textbox, "Name").(string)
		textRuns := make([]map[string]any, 0)
		for _, run := range textboxValues(textbox, ns) {
			value := textString(run.value)
			valueType := "text"
			if strings.HasPrefix(strings.TrimSpace(value), "=") {
				valueType = "expression"
			}
			textRuns = append(textRuns, map[string]any{
				"paragraph_index": run.paragraphIndex,
				"text_run_index":  run.textRunIndex,
				"value":           value,
				"value_type":      valueType,
				"style":           textboxStyle(run.element, ns),
			})
		}
		geometry := make(map[string]any)
		for _, property := range []string{"Top", "Left", "Height", "Width"} {
			if value := findChild(textbox, property, ns); value != nil {
				geometry[strings.ToLower(property)] = textString(value)
			}
		}
		textboxes = append(textboxes, map[string]any{
			"name":      name,
			"location":  elementLocation(root, textbox),
			"geometry":  geometry,
			"style":     textboxStyle(textbox, ns),
			"text_runs": textRuns,
		})
	}
	return textboxes
}

func textboxValues(textbox *Element, ns string) []textboxValue {
	values := make([]textboxValue, 0)
	paragraphs := findChildren(findChild(textbox, "Paragraphs", ns), "Paragraph", ns)
	for paragraphIndex, paragraph := range paragraphs {
		textRunIndex := 0
		for _, textRuns := range findChildren(paragraph, "TextRuns", ns) {
			for _, textRun := range findChildren(textRuns, "TextRun", ns) {
				values = append(values, textboxValue{
					element:        textRun,
					value:          findChild(textRun, "Value", ns),
					paragraphIndex: paragraphIndex,
					textRunIndex:   textRunIndex,
				})
				textRunIndex++
			}
		}
	}
	if len(values) != 0 {
		return values
	}
	for index, value := range findAll(textbox, "Value", ns) {
		values = append(values, textboxValue{value: value, paragraphIndex: -1, textRunIndex: index})
	}
	return values
}

func textboxStyle(element *Element, ns string) map[string]any {
	style := make(map[string]any)
	styleElement := findChild(element, "Style", ns)
	for _, property := range [][2]string{
		{"font_family", "FontFamily"},
		{"font_size", "FontSize"},
		{"font_weight", "FontWeight"},
		{"font_style", "FontStyle"},
		{"color", "Color"},
		{"background_color", "BackgroundColor"},
		{"text_align", "TextAlign"},
		{"vertical_align", "VerticalAlign"},
		{"format", "Format"},
	} {
		if value := findChild(styleElement, property[1], ns); value != nil {
			style[property[0]] = textString(value)
		}
	}
	return style
}

func elementLocation(root, target *Element) string {
	segments := []string{root.Name.Local}
	for element := target; element != nil && element != root; {
		parent := parentOf(root, element)
		if parent == nil {
			break
		}
		var segment strings.Builder
		segment.WriteString(element.Name.Local)
		if name, ok := attrValue(element, "Name").(string); ok && name != "" {
			segment.WriteString("[" + name + "]")
		} else {
			for index, sibling := range findChildren(parent, element.Name.Local, element.URI) {
				if sibling == element {
					segment.WriteString("[")
					segment.WriteString(strconv.Itoa(index))
					segment.WriteString("]")
					break
				}
			}
		}
		segments = append(segments, segment.String())
		element = parent
	}
	slices.Reverse(segments[1:])
	return "/" + strings.Join(segments, "/")
}

// CompareReports compares supported report sections against the first file.
func CompareReports(paths []string) (map[string]any, error) {
	if len(paths) < 2 {
		return nil, errors.New("compare RDL reports: at least two filepaths are required")
	}
	paths = slices.Clone(paths)
	baseline, err := reportSections(paths[0])
	if err != nil {
		return nil, err
	}
	comparisons := make([]map[string]any, 0, len(paths)-1)
	for _, path := range paths[1:] {
		current, err := reportSections(path)
		if err != nil {
			return nil, err
		}
		comparisons = append(comparisons, map[string]any{
			"filepath": path,
			"differences": map[string]any{
				"textboxes": diffItems(baseline["textboxes"], current["textboxes"], "location"),
				"datasets":  diffItems(baseline["datasets"], current["datasets"], "name"),
				"parameters": diffItems(
					baseline["parameters"], current["parameters"], "name",
				),
				"tablixes": diffItems(baseline["tablixes"], current["tablixes"], "name"),
				"columns":  diffItems(baseline["columns"], current["columns"], "key"),
			},
		})
	}
	return map[string]any{"baseline": paths[0], "comparisons": comparisons}, nil
}

func reportSections(path string) (map[string][]map[string]any, error) {
	doc, err := LoadXMLFile(path)
	if err != nil {
		return nil, err
	}
	datasets, err := Datasets(path, -1, "")
	if err != nil {
		return nil, err
	}
	parameters, err := Parameters(path)
	if err != nil {
		return nil, err
	}
	ns := doc.Root.URI
	tablixes := findAll(doc.Root, "Tablix", ns)
	columns := make([]map[string]any, 0)
	for _, tablix := range tablixes {
		name, _ := attrValue(tablix, "Name").(string)
		for _, column := range columnsForTablix(tablix, ns) {
			item := maps.Clone(column)
			item["tablix_name"] = name
			item["key"] = fmt.Sprintf("%s/%v", name, column["index"])
			columns = append(columns, item)
		}
	}
	return map[string][]map[string]any{
		"textboxes":  textboxesFromDocument(doc.Root, ns),
		"datasets":   datasets["datasets"].([]map[string]any),
		"parameters": parameters["parameters"].([]map[string]any),
		"tablixes":   tablixSummaries(doc.Root, ns),
		"columns":    columns,
	}, nil
}

func diffItems(before, after []map[string]any, identity string) []map[string]any {
	beforeByKey := make(map[string]map[string]any, len(before))
	for _, item := range before {
		beforeByKey[fmt.Sprint(item[identity])] = item
	}
	afterByKey := make(map[string]map[string]any, len(after))
	for _, item := range after {
		afterByKey[fmt.Sprint(item[identity])] = item
	}
	keys := make(map[string]struct{}, len(beforeByKey)+len(afterByKey))
	for key := range beforeByKey {
		keys[key] = struct{}{}
	}
	for key := range afterByKey {
		keys[key] = struct{}{}
	}
	changes := make([]map[string]any, 0)
	for _, key := range slices.Sorted(maps.Keys(keys)) {
		oldItem, hasOld := beforeByKey[key]
		newItem, hasNew := afterByKey[key]
		if hasOld && hasNew && reflect.DeepEqual(oldItem, newItem) {
			continue
		}
		change := map[string]any{"key": key}
		switch {
		case !hasOld:
			change["status"] = "added"
			change["after"] = newItem
		case !hasNew:
			change["status"] = "removed"
			change["before"] = oldItem
		default:
			change["status"] = "changed"
			change["before"] = oldItem
			change["after"] = newItem
		}
		changes = append(changes, change)
	}
	return changes
}

// PatchTextboxes previews or applies guarded static TextRun replacements.
func PatchTextboxes(paths []string, patches []TextboxPatch, dryRun bool) (map[string]any, error) {
	if len(paths) == 0 {
		return nil, errors.New("patch RDL textboxes: at least one filepath is required")
	}
	if len(patches) == 0 {
		return nil, errors.New("patch RDL textboxes: at least one patch is required")
	}
	paths = slices.Clone(paths)
	patches = slices.Clone(patches)
	for index, patch := range patches {
		if patch.ParagraphIndex != nil {
			value := *patch.ParagraphIndex
			patches[index].ParagraphIndex = &value
		}
		if patch.TextRunIndex != nil {
			value := *patch.TextRunIndex
			patches[index].TextRunIndex = &value
		}
	}
	type plannedFile struct {
		path        string
		doc         *Document
		resultIndex int
	}
	planned := make([]plannedFile, 0, len(paths))
	files := make([]map[string]any, 0, len(paths))
	conflicts := make([]map[string]any, 0)
	seenPaths := make(map[string]struct{}, len(paths))
	addConflict := func(path, textboxName, message string) {
		conflicts = append(conflicts, map[string]any{
			"filepath":     path,
			"textbox_name": textboxName,
			"message":      message,
		})
	}
	for _, path := range paths {
		fileResult := map[string]any{"filepath": path, "matched_count": 0, "changes": make([]map[string]any, 0)}
		cleanPath := filepath.Clean(path)
		if _, exists := seenPaths[cleanPath]; exists {
			addConflict(path, "", "filepath is repeated in the patch request")
			files = append(files, fileResult)
			continue
		}
		seenPaths[cleanPath] = struct{}{}
		doc, err := LoadXMLFile(path)
		if err != nil {
			addConflict(path, "", err.Error())
			fileResult["error"] = err.Error()
			files = append(files, fileResult)
			continue
		}
		planned = append(planned, plannedFile{path: path, doc: doc, resultIndex: len(files)})
		changes := make([]map[string]any, 0, len(patches))
		for _, patch := range patches {
			if patch.TextboxName == "" {
				addConflict(path, patch.TextboxName, "textbox_name is required")
				continue
			}
			matches := make([]*Element, 0, 1)
			for _, textbox := range findAll(doc.Root, "Textbox", doc.Root.URI) {
				if attrValue(textbox, "Name") == patch.TextboxName {
					matches = append(matches, textbox)
				}
			}
			if len(matches) != 1 {
				message := "textbox_name was not found"
				if len(matches) > 1 {
					message = "textbox_name is ambiguous"
				}
				addConflict(path, patch.TextboxName, message)
				continue
			}
			values := textboxValues(matches[0], doc.Root.URI)
			if (patch.ParagraphIndex == nil) != (patch.TextRunIndex == nil) {
				addConflict(path, patch.TextboxName, "paragraph_index and text_run_index must be supplied together")
				continue
			}
			var selected *textboxValue
			if patch.ParagraphIndex == nil {
				if len(values) == 1 {
					selected = &values[0]
				} else {
					addConflict(path, patch.TextboxName, "TextRun indexes are required for a Textbox with multiple values")
					continue
				}
			} else {
				for index := range values {
					if values[index].paragraphIndex == *patch.ParagraphIndex && values[index].textRunIndex == *patch.TextRunIndex {
						selected = &values[index]
						break
					}
				}
				if selected == nil {
					addConflict(path, patch.TextboxName, "TextRun indexes were not found")
					continue
				}
			}
			if selected.value == nil {
				addConflict(path, patch.TextboxName, "TextRun has no Value")
				continue
			}
			oldValue := textString(selected.value)
			if strings.HasPrefix(strings.TrimSpace(oldValue), "=") {
				addConflict(path, patch.TextboxName, "expression values cannot be patched")
				continue
			}
			if oldValue != patch.ExpectedValue {
				addConflict(path, patch.TextboxName, fmt.Sprintf("expected value %q, found %q", patch.ExpectedValue, oldValue))
				continue
			}
			if err := selected.value.SetText(patch.NewValue); err != nil {
				addConflict(path, patch.TextboxName, err.Error())
				continue
			}
			changes = append(changes, map[string]any{
				"textbox_name":    patch.TextboxName,
				"location":        elementLocation(doc.Root, matches[0]),
				"paragraph_index": selected.paragraphIndex,
				"text_run_index":  selected.textRunIndex,
				"old_value":       oldValue,
				"new_value":       patch.NewValue,
			})
		}
		fileResult["matched_count"] = len(changes)
		fileResult["changes"] = changes
		files = append(files, fileResult)
	}
	if len(conflicts) > 0 {
		return map[string]any{"success": false, "dry_run": dryRun, "files": files, "conflicts": conflicts}, nil
	}
	if !dryRun {
		for index, file := range planned {
			if err := file.doc.SaveXMLFile(file.path); err != nil {
				for _, completed := range planned[:index] {
					files[completed.resultIndex]["written"] = true
				}
				for _, remaining := range planned[index:] {
					files[remaining.resultIndex]["written"] = false
				}
				return map[string]any{
					"success":     false,
					"dry_run":     false,
					"partial":     index > 0,
					"failed_file": file.path,
					"write_error": fmt.Sprintf("save RDL %q: %v", file.path, err),
					"files":       files,
					"conflicts":   conflicts,
				}, nil
			}
			files[file.resultIndex]["written"] = true
		}
	}
	return map[string]any{"success": true, "dry_run": dryRun, "files": files, "conflicts": conflicts}, nil
}
