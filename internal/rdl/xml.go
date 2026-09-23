// Package rdl reads, validates, compares, and edits SSRS report definitions.
package rdl

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
)

// Document holds an RDL XML tree with original qualified names and other tokens.
type Document struct {
	Root   *Element
	before []xml.Token
	after  []xml.Token
}

// Element holds one XML element and its ordered content.
type Element struct {
	Name     xml.Name
	URI      string
	Attrs    []xml.Attr
	children []any
}

// ParseXML reads XML while retaining namespace prefixes, attributes, and unknown nodes.
func ParseXML(data []byte) (*Document, error) {
	validator := xml.NewDecoder(bytes.NewReader(data))
	for {
		_, err := validator.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("validate XML: %w", err)
		}
	}

	doc := &Document{}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []*Element
	var namespaces []map[string]string
	for {
		token, err := decoder.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse XML: %w", err)
		}
		switch value := xml.CopyToken(token).(type) {
		case xml.StartElement:
			prefixes := map[string]string{}
			if len(namespaces) > 0 {
				prefixes = maps.Clone(namespaces[len(namespaces)-1])
			}
			for _, attr := range value.Attr {
				if attr.Name.Space == "xmlns" {
					prefixes[attr.Name.Local] = attr.Value
				} else if attr.Name.Local == "xmlns" && attr.Name.Space == "" {
					prefixes[""] = attr.Value
				}
			}
			element := &Element{Name: value.Name, URI: prefixes[value.Name.Space], Attrs: value.Attr}
			if len(stack) == 0 {
				if doc.Root != nil {
					return nil, errors.New("multiple XML roots")
				}
				doc.Root = element
			} else {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, element)
			}
			stack = append(stack, element)
			namespaces = append(namespaces, prefixes)
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1].Name != value.Name {
				return nil, errors.New("mismatched XML end element")
			}
			stack = stack[:len(stack)-1]
			namespaces = namespaces[:len(namespaces)-1]
		default:
			if len(stack) > 0 {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, value)
			} else if doc.Root == nil {
				doc.before = append(doc.before, value)
			} else {
				doc.after = append(doc.after, value)
			}
		}
	}
	if doc.Root == nil || len(stack) != 0 {
		return nil, errors.New("missing or unclosed XML root")
	}
	return doc, nil
}

// LoadXMLFile reads an RDL XML file into a document.
func LoadXMLFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read RDL %q: %w", path, err)
	}
	return ParseXML(data)
}

// Child finds the first direct child by local element name.
func (element *Element) Child(local string) *Element {
	for _, child := range element.children {
		if node, ok := child.(*Element); ok && node.Name.Local == local {
			return node
		}
	}
	return nil
}

// SetText changes a leaf element's text without removing comments or other tokens.
func (element *Element) SetText(text string) error {
	for _, child := range element.children {
		if _, ok := child.(*Element); ok {
			return errors.New("cannot set text on element with child elements")
		}
	}
	content := make([]any, 0, len(element.children)+1)
	replaced := false
	for _, child := range element.children {
		if _, ok := child.(xml.CharData); ok {
			if !replaced {
				content = append(content, xml.CharData(text))
				replaced = true
			}
			continue
		}
		content = append(content, child)
	}
	if !replaced {
		content = append([]any{xml.CharData(text)}, content...)
	}
	element.children = content
	return nil
}

// EncodeXML encodes the tree and verifies that it remains valid XML.
func (doc *Document) EncodeXML() ([]byte, error) {
	if doc == nil || doc.Root == nil {
		return nil, errors.New("missing XML root")
	}
	output := &bytes.Buffer{}
	before := doc.before
	if len(before) > 0 {
		if bom, ok := before[0].(xml.CharData); ok && bytes.Equal(bom, []byte{0xef, 0xbb, 0xbf}) {
			output = bytes.NewBuffer(bom)
			before = before[1:]
		}
	}
	encoder := xml.NewEncoder(output)
	for _, token := range before {
		if err := encoder.EncodeToken(token); err != nil {
			return nil, fmt.Errorf("encode XML prolog: %w", err)
		}
	}
	if err := writeElement(encoder, doc.Root); err != nil {
		return nil, err
	}
	for _, token := range doc.after {
		if err := encoder.EncodeToken(token); err != nil {
			return nil, fmt.Errorf("encode XML epilog: %w", err)
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("flush XML: %w", err)
	}
	validator := xml.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		_, err := validator.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("validate encoded XML: %w", err)
		}
	}
	return output.Bytes(), nil
}

// SaveXMLFile verifies the edited XML and replaces an existing RDL file.
func (doc *Document) SaveXMLFile(path string) error {
	data, err := doc.EncodeXML()
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat RDL %q: %w", path, err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".rdl-*")
	if err != nil {
		return fmt.Errorf("create RDL temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(file.Name()) // The rename removes it on success.
	}()
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return errors.Join(fmt.Errorf("set RDL permissions: %w", err), file.Close())
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(fmt.Errorf("write RDL temp file: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close RDL temp file: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace RDL %q: %w", path, err)
	}
	return nil
}

func writeElement(encoder *xml.Encoder, element *Element) error {
	start := xml.StartElement{Name: qualifiedName(element.Name), Attr: make([]xml.Attr, len(element.Attrs))}
	for i, attr := range element.Attrs {
		start.Attr[i] = xml.Attr{Name: qualifiedName(attr.Name), Value: attr.Value}
	}
	if err := encoder.EncodeToken(start); err != nil {
		return fmt.Errorf("encode element %q: %w", element.Name.Local, err)
	}
	for _, child := range element.children {
		if node, ok := child.(*Element); ok {
			if err := writeElement(encoder, node); err != nil {
				return err
			}
			continue
		}
		if err := encoder.EncodeToken(child.(xml.Token)); err != nil {
			return fmt.Errorf("encode content of %q: %w", element.Name.Local, err)
		}
	}
	if err := encoder.EncodeToken(start.End()); err != nil {
		return fmt.Errorf("end element %q: %w", element.Name.Local, err)
	}
	return nil
}

func qualifiedName(name xml.Name) xml.Name {
	if name.Space == "" {
		return name
	}
	return xml.Name{Local: name.Space + ":" + name.Local}
}
