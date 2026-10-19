package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func build(t *testing.T, fill func(*Document)) map[string]string {
	t.Helper()
	var d Document
	fill(&d)

	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}

	parts := map[string]string{}
	for _, f := range z.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}
	return parts
}

func wellFormed(t *testing.T, name, content string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(content))
	for {
		if _, err := dec.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("%s is not well-formed XML: %v", name, err)
		}
	}
}

func TestWritesEveryRequiredPartAsValidXML(t *testing.T) {
	parts := build(t, func(d *Document) { d.Title("RFP") })
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/_rels/document.xml.rels", "word/styles.xml", "word/document.xml"} {
		content, ok := parts[name]
		if !ok {
			t.Fatalf("missing part %s", name)
		}
		wellFormed(t, name, content)
	}
}

func TestEscapesUserText(t *testing.T) {
	doc := build(t, func(d *Document) {
		d.Heading(`Q1: "A & B" <script>`)
		d.Paragraph("bad control char \x01 here")
	})["word/document.xml"]

	wellFormed(t, "document.xml", doc)
	if !strings.Contains(doc, "&amp; B") || strings.Contains(doc, "<script>") {
		t.Fatalf("text not escaped: %s", doc)
	}
}

func TestParagraphsAndLineBreaks(t *testing.T) {
	doc := build(t, func(d *Document) { d.Paragraph("First line\nsame paragraph\n\nSecond paragraph\n\n\n") })["word/document.xml"]

	if got := strings.Count(doc, "<w:p>"); got != 2 {
		t.Fatalf("paragraphs = %d, want 2 (blank lines split, trailing blanks dropped)", got)
	}
	if !strings.Contains(doc, "First line</w:t><w:br/>") {
		t.Fatalf("single newline should become a line break: %s", doc)
	}
}

func TestStylesAreDefined(t *testing.T) {
	parts := build(t, func(d *Document) {
		d.Title("t")
		d.Subtitle("s")
		d.Heading("h")
		d.Note("n")
		d.Placeholder("p")
	})
	for _, style := range []string{styleTitle, styleSubtitle, styleHeading, styleNote} {
		if !strings.Contains(parts["word/document.xml"], `w:val="`+style+`"`) {
			t.Fatalf("style %s not used", style)
		}
		if !strings.Contains(parts["word/styles.xml"], `w:styleId="`+style+`"`) {
			t.Fatalf("style %s used but not defined", style)
		}
	}
	if !strings.Contains(parts["word/document.xml"], "<w:i/>") {
		t.Fatal("placeholder should be italic")
	}
}
