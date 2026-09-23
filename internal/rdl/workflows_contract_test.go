package rdl

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowFixtureRoundTrip(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "rdl", "workflow_report.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	first, err := ParseXML(data)
	if err != nil {
		t.Fatalf("ParseXML(%q) error = %v, want nil", path, err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML(%q) error = %v, want nil", path, err)
	}
	second, err := ParseXML(encoded)
	if err != nil {
		t.Fatalf("ParseXML(EncodeXML(%q)) error = %v, want nil", path, err)
	}
	reencoded, err := second.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML(ParseXML(EncodeXML(%q))) error = %v, want nil", path, err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Errorf("EncodeXML(ParseXML(EncodeXML(%q))) = %s, want stable XML %s", path, reencoded, encoded)
	}
	if got := len(findAll(first.Root, "Textbox", first.Root.URI)); got != 10 {
		t.Errorf("workflow fixture Textboxes = %d, want 10", got)
	}
	if got := len(findAll(first.Root, "Tablix", first.Root.URI)); got != 2 {
		t.Errorf("workflow fixture Tablix = %d, want 2", got)
	}
	if !bytes.Contains(encoded, []byte("ext:Untouched")) || !bytes.Contains(encoded, []byte("urn:rdl-mcp-test")) {
		t.Errorf("EncodeXML(%q) = %s, want namespaced extension element retained", path, encoded)
	}
}
