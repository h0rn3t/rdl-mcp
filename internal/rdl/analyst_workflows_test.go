package rdl

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func workflowFixture(t *testing.T) string {
	t.Helper()
	fixture := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", fixture, err)
	}
	return writeWorkflowFixture(t, data)
}

func writeWorkflowFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.rdl")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}

func workflowTextbox(t *testing.T, result map[string]any, name string) map[string]any {
	t.Helper()
	textboxes, ok := result["textboxes"].([]map[string]any)
	if !ok {
		t.Fatalf("textboxes = %T, want []map[string]any", result["textboxes"])
	}
	for _, textbox := range textboxes {
		if textbox["name"] == name {
			return textbox
		}
	}
	t.Fatalf("textboxes has no name %q", name)
	return nil
}

func workflowRunValues(t *testing.T, textbox map[string]any) []string {
	t.Helper()
	runs, ok := textbox["text_runs"].([]map[string]any)
	if !ok {
		t.Fatalf("%s.text_runs = %T, want []map[string]any", textbox["name"], textbox["text_runs"])
	}
	values := make([]string, 0, len(runs))
	for _, run := range runs {
		value, ok := run["value"].(string)
		if !ok {
			t.Fatalf("%s TextRun value = %T, want string", textbox["name"], run["value"])
		}
		values = append(values, value)
	}
	return values
}

func TestTextboxesReturnsNamedValuesLocationGeometryAndStyle(t *testing.T) {
	path := workflowFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	got, err := Textboxes(path)
	if err != nil {
		t.Fatalf("Textboxes(%q) error = %v, want nil", path, err)
	}
	textboxes, ok := got["textboxes"].([]map[string]any)
	if !ok || len(textboxes) != 10 {
		t.Fatalf("Textboxes(%q) count = %d, want 10", path, len(textboxes))
	}
	title := workflowTextbox(t, got, "Title")
	if values := workflowRunValues(t, title); len(values) != 1 || values[0] != "Monthly Revenue" {
		t.Errorf("Title TextRuns = %v, want [Monthly Revenue]", values)
	}
	if title["location"] == "" || !strings.Contains(title["location"].(string), "Title") {
		t.Errorf("Title location = %v, want a path containing Title", title["location"])
	}
	geometry := title["geometry"].(map[string]any)
	if geometry["top"] != "0.2in" || geometry["width"] != "4in" {
		t.Errorf("Title geometry = %#v, want top 0.2in and width 4in", geometry)
	}
	style := title["style"].(map[string]any)
	if style["color"] != "DarkBlue" || style["text_align"] != "Left" {
		t.Errorf("Title style = %#v, want DarkBlue and Left", style)
	}
	rich := workflowTextbox(t, got, "RichTitle")
	if values := workflowRunValues(t, rich); len(values) != 2 || values[0] != "Revenue" || values[1] != " period" {
		t.Errorf("RichTitle TextRuns = %v, want [Revenue,  period]", values)
	}
	pageCaption := workflowTextbox(t, got, "PageCaption")
	if !strings.Contains(pageCaption["location"].(string), "PageHeader") {
		t.Errorf("PageCaption location = %v, want PageHeader", pageCaption["location"])
	}
	headerName := workflowTextbox(t, got, "HeaderName")
	if !strings.Contains(headerName["location"].(string), "MainTable") {
		t.Errorf("HeaderName location = %v, want MainTable", headerName["location"])
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) after Textboxes() error = %v, want nil", path, err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("Textboxes(%q) changed the source file", path)
	}
}

func TestCompareReportsReturnsSemanticSectionDiff(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", fixture, err)
	}
	variant := bytes.Replace(data, []byte("Monthly Revenue"), []byte("Quarterly Revenue"), 1)
	variant = bytes.Replace(variant, []byte("dbo.usp_report_month"), []byte("dbo.usp_report_quarter"), 1)
	variant = bytes.Replace(variant, []byte("Start date"), []byte("Quarter start"), 1)
	variant = bytes.Replace(variant, []byte("<Width>1in</Width>"), []byte("<Width>1.25in</Width>"), 1)
	variant = bytes.Replace(variant, []byte("<DataSetName>MainDataset</DataSetName>"), []byte("<DataSetName>SummaryDataset</DataSetName>"), 1)
	basePath := writeWorkflowFixture(t, data)
	variantPath := writeWorkflowFixture(t, variant)
	got, err := CompareReports([]string{basePath, variantPath})
	if err != nil {
		t.Fatalf("CompareReports([%q %q]) error = %v, want nil", basePath, variantPath, err)
	}
	comparisons, ok := got["comparisons"].([]map[string]any)
	if !ok || len(comparisons) != 1 {
		t.Fatalf("CompareReports() comparisons = %#v, want one comparison", got["comparisons"])
	}
	differences := comparisons[0]["differences"].(map[string]any)
	for _, category := range []string{"textboxes", "datasets", "parameters", "tablixes", "columns"} {
		items, ok := differences[category].([]map[string]any)
		if !ok || len(items) == 0 {
			t.Errorf("CompareReports() %s differences = %#v, want at least one", category, differences[category])
		}
	}
}

func TestCompareReportsIgnoresFormattingWhitespace(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", fixture, err)
	}
	formatted := bytes.Replace(data, []byte("<DataSources>"), []byte("<DataSources>\n    "), 1)
	basePath := writeWorkflowFixture(t, data)
	formattedPath := writeWorkflowFixture(t, formatted)
	got, err := CompareReports([]string{basePath, formattedPath})
	if err != nil {
		t.Fatalf("CompareReports([%q %q]) error = %v, want nil", basePath, formattedPath, err)
	}
	comparisons := got["comparisons"].([]map[string]any)
	differences := comparisons[0]["differences"].(map[string]any)
	for _, category := range []string{"textboxes", "datasets", "parameters", "tablixes", "columns"} {
		if items := differences[category].([]map[string]any); len(items) != 0 {
			t.Errorf("CompareReports() %s differences = %#v, want none for formatting-only change", category, items)
		}
	}
}

func TestPatchTextboxesDryRunAndApply(t *testing.T) {
	path := workflowFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	patches := []TextboxPatch{{TextboxName: "Title", ExpectedValue: "Monthly Revenue", NewValue: "Quarterly Revenue"}}
	preview, err := PatchTextboxes([]string{path}, patches, true)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q], patches, true) error = %v, want nil", path, err)
	}
	if preview["success"] != true || preview["dry_run"] != true {
		t.Errorf("PatchTextboxes dry run = %#v, want success and dry_run true", preview)
	}
	files := preview["files"].([]map[string]any)
	if len(files) != 1 || files[0]["matched_count"] != 1 || len(files[0]["changes"].([]map[string]any)) != 1 {
		t.Errorf("PatchTextboxes dry run files = %#v, want one match and diff", files)
	}
	afterPreview, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) after dry run error = %v, want nil", path, err)
	}
	if !bytes.Equal(afterPreview, before) {
		t.Errorf("PatchTextboxes(%q, dry_run=true) changed the source file", path)
	}
	applied, err := PatchTextboxes([]string{path}, patches, false)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q], patches, false) error = %v, want nil", path, err)
	}
	appliedFiles := applied["files"].([]map[string]any)
	if applied["success"] != true || len(appliedFiles) != 1 || appliedFiles[0]["written"] != true {
		t.Errorf("PatchTextboxes apply result = %#v, want one written file", applied)
	}
	updated, err := Textboxes(path)
	if err != nil {
		t.Fatalf("Textboxes(%q) after patch error = %v, want nil", path, err)
	}
	if values := workflowRunValues(t, workflowTextbox(t, updated, "Title")); len(values) != 1 || values[0] != "Quarterly Revenue" {
		t.Errorf("patched Title TextRuns = %v, want [Quarterly Revenue]", values)
	}
	if values := workflowRunValues(t, workflowTextbox(t, updated, "RichTitle")); len(values) != 2 || values[0] != "Revenue" || values[1] != " period" {
		t.Errorf("unrelated RichTitle TextRuns = %v, want [Revenue,  period]", values)
	}
}

func TestPatchTextboxesPreflightsAllFilesAndTextRuns(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", fixture, err)
	}
	firstPath := writeWorkflowFixture(t, data)
	changed := bytes.Replace(data, []byte("Monthly Revenue"), []byte("Other Revenue"), 1)
	secondPath := writeWorkflowFixture(t, changed)
	firstBefore, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", firstPath, err)
	}
	secondBefore, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", secondPath, err)
	}
	patches := []TextboxPatch{{TextboxName: "Title", ExpectedValue: "Monthly Revenue", NewValue: "Quarterly Revenue"}}
	result, err := PatchTextboxes([]string{firstPath, secondPath}, patches, false)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q %q], patches, false) error = %v, want conflict result", firstPath, secondPath, err)
	}
	if result["success"] != false || len(result["conflicts"].([]map[string]any)) != 1 {
		t.Errorf("PatchTextboxes conflict result = %#v, want one conflict", result)
	}
	firstAfter, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after conflict error = %v, want nil", firstPath, err)
	}
	secondAfter, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after conflict error = %v, want nil", secondPath, err)
	}
	if !bytes.Equal(firstAfter, firstBefore) || !bytes.Equal(secondAfter, secondBefore) {
		t.Errorf("PatchTextboxes conflict changed a file: first=%t second=%t", !bytes.Equal(firstAfter, firstBefore), !bytes.Equal(secondAfter, secondBefore))
	}
	expressionPatch := []TextboxPatch{{TextboxName: "DataName", ExpectedValue: "=Fields!Name.Value", NewValue: "Name"}}
	expressionResult, err := PatchTextboxes([]string{firstPath}, expressionPatch, false)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q], expression patch, false) error = %v, want conflict result", firstPath, err)
	}
	if expressionResult["success"] != false {
		t.Errorf("PatchTextboxes expression result = %#v, want success false", expressionResult)
	}
	unindexedRichPatch := []TextboxPatch{{TextboxName: "RichTitle", ExpectedValue: "Revenue", NewValue: "Net revenue"}}
	unindexedResult, err := PatchTextboxes([]string{firstPath}, unindexedRichPatch, false)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q], unindexed TextRun patch, false) error = %v, want conflict result", firstPath, err)
	}
	if unindexedResult["success"] != false {
		t.Errorf("PatchTextboxes unindexed result = %#v, want success false", unindexedResult)
	}
	unchanged, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after rejected patches error = %v, want nil", firstPath, err)
	}
	if !bytes.Equal(unchanged, firstBefore) {
		t.Errorf("PatchTextboxes rejected patch changed %q", firstPath)
	}
	zero, one := 0, 1
	richPatch := []TextboxPatch{{TextboxName: "RichTitle", ExpectedValue: " period", NewValue: " quarter", ParagraphIndex: &zero, TextRunIndex: &one}}
	if _, err := PatchTextboxes([]string{firstPath}, richPatch, false); err != nil {
		t.Fatalf("PatchTextboxes([%q], indexed TextRun patch, false) error = %v, want nil", firstPath, err)
	}
	updated, err := Textboxes(firstPath)
	if err != nil {
		t.Fatalf("Textboxes(%q) after indexed patch error = %v, want nil", firstPath, err)
	}
	if values := workflowRunValues(t, workflowTextbox(t, updated, "RichTitle")); len(values) != 2 || values[0] != "Revenue" || values[1] != " quarter" {
		t.Errorf("patched RichTitle TextRuns = %v, want [Revenue,  quarter]", values)
	}
}

func TestPatchTextboxesReportsPartialWriteStatus(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", fixture, err)
	}
	firstDir := filepath.Join(t.TempDir(), "first")
	secondDir := filepath.Join(t.TempDir(), "second")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("Mkdir(%q) error = %v, want nil", dir, err)
		}
	}
	firstPath := filepath.Join(firstDir, "report.rdl")
	secondPath := filepath.Join(secondDir, "report.rdl")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
		}
	}
	if err := os.Chmod(secondDir, 0o500); err != nil {
		t.Fatalf("Chmod(%q, 0500) error = %v, want nil", secondDir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(secondDir, 0o700) })
	patches := []TextboxPatch{{TextboxName: "Title", ExpectedValue: "Monthly Revenue", NewValue: "Quarterly Revenue"}}
	result, err := PatchTextboxes([]string{firstPath, secondPath}, patches, false)
	if err != nil {
		t.Fatalf("PatchTextboxes([%q %q], patches, false) error = %v, want per-file result", firstPath, secondPath, err)
	}
	if result["success"] != false || result["partial"] != true {
		t.Errorf("PatchTextboxes partial result = %#v, want success false and partial true", result)
	}
	if result["write_error"] == "" {
		t.Errorf("PatchTextboxes partial result = %#v, want write_error", result)
	}
	files := result["files"].([]map[string]any)
	if len(files) != 2 || files[0]["written"] != true || files[1]["written"] != false {
		t.Errorf("PatchTextboxes file status = %#v, want written true then false", files)
	}
	if err := os.Chmod(secondDir, 0o700); err != nil {
		t.Fatalf("Chmod(%q, 0700) after patch error = %v, want nil", secondDir, err)
	}
	first, err := Textboxes(firstPath)
	if err != nil {
		t.Fatalf("Textboxes(%q) after partial write error = %v, want nil", firstPath, err)
	}
	second, err := Textboxes(secondPath)
	if err != nil {
		t.Fatalf("Textboxes(%q) after partial write error = %v, want nil", secondPath, err)
	}
	if values := workflowRunValues(t, workflowTextbox(t, first, "Title")); values[0] != "Quarterly Revenue" {
		t.Errorf("first file Title = %v, want changed value", values)
	}
	if values := workflowRunValues(t, workflowTextbox(t, second, "Title")); values[0] != "Monthly Revenue" {
		t.Errorf("second file Title = %v, want original value", values)
	}
}

func TestColumnToolsRequireNameForMultipleTablix(t *testing.T) {
	path := workflowFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	description, err := Describe(path)
	if err != nil {
		t.Fatalf("Describe(%q) error = %v, want nil", path, err)
	}
	tablixes := description["tablixes"].([]map[string]any)
	if len(tablixes) != 2 || tablixes[0]["name"] != "MainTable" || tablixes[1]["name"] != "SummaryTable" {
		t.Errorf("Describe(%q).tablixes = %#v, want both named Tablix", path, tablixes)
	}
	columns, err := Columns(path, "")
	if err != nil {
		t.Fatalf("Columns(%q, %q) error = %v, want in-band ambiguity", path, "", err)
	}
	if !strings.Contains(columns["error"].(string), "tablix_name") {
		t.Errorf("Columns(%q, empty name) error = %v, want tablix_name guidance", path, columns["error"])
	}
	result, err := UpdateColumnWidth(path, 0, "3in", "")
	if err != nil {
		t.Fatalf("UpdateColumnWidth(%q, 0, 3in, empty name) error = %v, want in-band ambiguity", path, err)
	}
	if result["success"] != false {
		t.Errorf("UpdateColumnWidth(%q, empty name) = %#v, want success false", path, result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) after ambiguous mutation error = %v, want nil", path, err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("UpdateColumnWidth(%q, empty name) changed the file", path)
	}
	selected, err := Columns(path, "SummaryTable")
	if err != nil {
		t.Fatalf("Columns(%q, SummaryTable) error = %v, want nil", path, err)
	}
	if len(selected["columns"].([]map[string]any)) != 1 {
		t.Errorf("Columns(%q, SummaryTable) count = %d, want 1", path, len(selected["columns"].([]map[string]any)))
	}
	if _, err := UpdateColumnWidth(path, 0, "3in", "SummaryTable"); err != nil {
		t.Fatalf("UpdateColumnWidth(%q, SummaryTable) error = %v, want nil", path, err)
	}
	mainTable, err := Columns(path, "MainTable")
	if err != nil {
		t.Fatalf("Columns(%q, MainTable) error = %v, want nil", path, err)
	}
	if mainTable["columns"].([]map[string]any)[0]["width"] != "1in" {
		t.Errorf("MainTable first column width = %v, want unchanged 1in", mainTable["columns"].([]map[string]any)[0]["width"])
	}
}

func TestColumnMutationsTargetSelectedTablix(t *testing.T) {
	t.Run("header", func(t *testing.T) {
		path := workflowFixture(t)
		if _, err := UpdateColumnHeader(path, "Total", "Summary total", "SummaryTable"); err != nil {
			t.Fatalf("UpdateColumnHeader(%q, SummaryTable) error = %v, want nil", path, err)
		}
		columns, err := Columns(path, "SummaryTable")
		if err != nil {
			t.Fatalf("Columns(%q, SummaryTable) error = %v, want nil", path, err)
		}
		if got := columns["columns"].([]map[string]any)[0]["header"]; got != "Summary total" {
			t.Errorf("SummaryTable header = %v, want Summary total", got)
		}
	})
	t.Run("format", func(t *testing.T) {
		path := workflowFixture(t)
		if _, err := UpdateColumnFormat(path, 0, "C0", "SummaryTable"); err != nil {
			t.Fatalf("UpdateColumnFormat(%q, SummaryTable) error = %v, want nil", path, err)
		}
		doc, err := LoadXMLFile(path)
		if err != nil {
			t.Fatalf("LoadXMLFile(%q) error = %v, want nil", path, err)
		}
		var textbox *Element
		for _, candidate := range findAll(findAll(doc.Root, "Tablix", doc.Root.URI)[1], "Textbox", doc.Root.URI) {
			if attrValue(candidate, "Name") == "SummaryValue" {
				textbox = candidate
				break
			}
		}
		if got := textString(findChild(findChild(findFirst(textbox, "TextRun", doc.Root.URI), "Style", doc.Root.URI), "Format", doc.Root.URI)); got != "C0" {
			t.Errorf("SummaryTable format = %q, want C0", got)
		}
	})
	t.Run("add and remove", func(t *testing.T) {
		path := workflowFixture(t)
		if _, err := AddColumn(path, -1, "New", "Total", nil, nil, nil, "SummaryTable"); err != nil {
			t.Fatalf("AddColumn(%q, SummaryTable) error = %v, want nil", path, err)
		}
		columns, err := Columns(path, "SummaryTable")
		if err != nil || len(columns["columns"].([]map[string]any)) != 2 {
			t.Fatalf("Columns(%q, SummaryTable) after add = %#v, %v; want two columns", path, columns, err)
		}
		if _, err := RemoveColumn(path, 1, true, "SummaryTable"); err != nil {
			t.Fatalf("RemoveColumn(%q, SummaryTable) error = %v, want nil", path, err)
		}
		columns, err = Columns(path, "SummaryTable")
		if err != nil || len(columns["columns"].([]map[string]any)) != 1 {
			t.Fatalf("Columns(%q, SummaryTable) after remove = %#v, %v; want one column", path, columns, err)
		}
	})
}

func TestValidateChecksTextboxExpressionsOutsideTablix(t *testing.T) {
	path := workflowFixture(t)
	got := Validate(path)
	if got["valid"] != false {
		t.Errorf("Validate(%q).valid = %v, want false for an unknown outside field", path, got["valid"])
	}
	issues, ok := got["issues"].([]string)
	if !ok {
		t.Fatalf("Validate(%q).issues = %T, want []string", path, got["issues"])
	}
	issueText := strings.Join(issues, "\n")
	if !strings.Contains(issueText, "OutsideExpression") || !strings.Contains(issueText, "MissingOutside") {
		t.Errorf("Validate(%q).issues = %q, want OutsideExpression and MissingOutside", path, issueText)
	}
	if got["validation_scope"] != "static_rdl" {
		t.Errorf("Validate(%q).validation_scope = %v, want static_rdl", path, got["validation_scope"])
	}
	notChecked, ok := got["not_checked"].([]string)
	if !ok {
		t.Fatalf("Validate(%q).not_checked = %T, want []string", path, got["not_checked"])
	}
	if !slices.Contains(notChecked, "sql_execution") || !slices.Contains(notChecked, "ssrs_render") {
		t.Errorf("Validate(%q).not_checked = %v, want SQL and SSRS render", path, notChecked)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	unknownScope := bytes.Replace(data, []byte(`=First(Fields!MissingOutside.Value, "MainDataset")`), []byte("=First(Fields!MissingOutside.Value)"), 1)
	unknownPath := writeWorkflowFixture(t, unknownScope)
	unknown := Validate(unknownPath)
	if unknown["valid"] != true {
		t.Errorf("Validate(%q).valid = %v, want true when outside field scope is unknown", unknownPath, unknown["valid"])
	}
	warningList, ok := unknown["warnings"].([]string)
	if !ok {
		t.Fatalf("Validate(%q).warnings = %T, want []string", unknownPath, unknown["warnings"])
	}
	warnings := strings.Join(warningList, "\n")
	if !strings.Contains(warnings, "OutsideExpression") {
		t.Errorf("Validate(%q).warnings = %q, want unknown Textbox scope warning", unknownPath, warnings)
	}
}
