package rdl

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const namespaceFixture = `<?xml version="1.0"?><Report xmlns="http://schemas.microsoft.com/sqlserver/reporting/2016/01/reportdefinition" xmlns:rd="http://schemas.microsoft.com/SQLServer/reporting/reportdesigner" xmlns:ext="urn:vendor" ext:flag="yes"><!--keep--><Title>Old</Title><ext:Extra ext:key="value" xmlns:other="urn:other"><ext:Nested>untouched</ext:Nested><other:X>nested namespace</other:X></ext:Extra><rd:ReportID>123</rd:ReportID></Report>`

func TestXMLPointEditPreservesOtherContent(t *testing.T) {
	doc, err := ParseXML([]byte(namespaceFixture))
	if err != nil {
		t.Fatalf("ParseXML(namespaceFixture) error = %v, want nil", err)
	}
	title := doc.Root.Child("Title")
	if title == nil {
		t.Fatal("Root.Child(Title) = nil, want Title")
	}
	if err := title.SetText("New & safe"); err != nil {
		t.Fatalf("Title.SetText() error = %v, want nil", err)
	}
	got, err := doc.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML() error = %v, want nil", err)
	}
	for _, want := range []string{"xmlns:rd=", "xmlns:ext=", "ext:flag=", "<ext:Extra", "xmlns:other=", "<other:X>nested namespace</other:X>", "<ext:Nested>untouched</ext:Nested>", "<rd:ReportID>123</rd:ReportID>", "<!--keep-->", "<Title>New &amp; safe</Title>"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("EncodeXML() = %s, want substring %q", got, want)
		}
	}
	if !sameElementsExceptTitle(t, []byte(namespaceFixture), got) {
		t.Errorf("EncodeXML() changed non-target XML: %s", got)
	}
}

func TestXMLFilePointEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.rdl")
	if err := os.WriteFile(path, []byte(namespaceFixture), 0o640); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	doc, err := LoadXMLFile(path)
	if err != nil {
		t.Fatalf("LoadXMLFile(%q) error = %v, want nil", path, err)
	}
	if err := doc.Root.Child("Title").SetText("Changed"); err != nil {
		t.Fatalf("Title.SetText() error = %v, want nil", err)
	}
	if err := doc.SaveXMLFile(path); err != nil {
		t.Fatalf("SaveXMLFile(%q) error = %v, want nil", path, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	if !bytes.Contains(got, []byte("<Title>Changed</Title>")) || !bytes.Contains(got, []byte("<other:X>nested namespace</other:X>")) {
		t.Errorf("SaveXMLFile(%q) = %s, want edited title and unknown namespace", path, got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v, want nil", path, err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("Stat(%q).Mode() = %v, want 0640", path, info.Mode().Perm())
	}
}

func TestXMLRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{"<Report><Title></Report>", `<Report xmlns:x="urn:x"><x:Title></Title></Report>`, ""} {
		if _, err := ParseXML([]byte(input)); err == nil {
			t.Errorf("ParseXML(%q) error = nil, want error", input)
		}
	}
}

func sameElementsExceptTitle(t *testing.T, before, after []byte) bool {
	t.Helper()
	read := func(data []byte) []xml.Token {
		decoder := xml.NewDecoder(bytes.NewReader(data))
		var tokens []xml.Token
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Decoder.Token() error = %v, want nil", err)
			}
			if text, ok := token.(xml.CharData); ok && (string(text) == "Old" || string(text) == "New & safe") {
				continue
			}
			tokens = append(tokens, xml.CopyToken(token))
		}
		return tokens
	}
	left, right := read(before), read(after)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !tokensEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func tokensEqual(a, b xml.Token) bool {
	switch x := a.(type) {
	case xml.StartElement:
		y, ok := b.(xml.StartElement)
		if !ok || x.Name != y.Name || len(x.Attr) != len(y.Attr) {
			return false
		}
		for i := range x.Attr {
			if x.Attr[i] != y.Attr[i] {
				return false
			}
		}
		return true
	case xml.EndElement:
		y, ok := b.(xml.EndElement)
		return ok && x.Name == y.Name
	case xml.CharData:
		y, ok := b.(xml.CharData)
		return ok && string(x) == string(y)
	case xml.Comment:
		y, ok := b.(xml.Comment)
		return ok && string(x) == string(y)
	case xml.ProcInst:
		y, ok := b.(xml.ProcInst)
		return ok && x.Target == y.Target && string(x.Inst) == string(y.Inst)
	case xml.Directive:
		y, ok := b.(xml.Directive)
		return ok && string(x) == string(y)
	}
	return false
}
