// Package docx writes simple Word documents: a title, headings, paragraphs
// and small notes. A .docx file is a zip of XML parts; this writes the five
// parts Word needs and nothing else, so it has no dependencies.
package docx

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strings"
)

// Paragraph styles defined in styles.xml.
const (
	styleTitle    = "Title"
	styleSubtitle = "Subtitle"
	styleHeading  = "Heading1"
	styleNote     = "Note"
)

// Document accumulates paragraphs; call Write to produce the file.
type Document struct {
	body strings.Builder
}

func (d *Document) Title(text string)    { d.paragraph(styleTitle, text, false) }
func (d *Document) Subtitle(text string) { d.paragraph(styleSubtitle, text, false) }
func (d *Document) Heading(text string)  { d.paragraph(styleHeading, text, false) }

// Paragraph adds body text. Blank lines start a new paragraph; single
// newlines become line breaks within one.
func (d *Document) Paragraph(text string) {
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(para) != "" {
			d.paragraph("", para, false)
		}
	}
}

// Note adds small grey text, e.g. sources or review status.
func (d *Document) Note(text string) { d.paragraph(styleNote, text, false) }

// Placeholder adds italic body text standing in for missing content.
func (d *Document) Placeholder(text string) { d.paragraph("", text, true) }

func (d *Document) paragraph(style, text string, italic bool) {
	d.body.WriteString("<w:p>")
	if style != "" {
		d.body.WriteString(`<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`)
	}
	d.body.WriteString("<w:r>")
	if italic {
		d.body.WriteString("<w:rPr><w:i/></w:rPr>")
	}
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			d.body.WriteString("<w:br/>")
		}
		d.body.WriteString(`<w:t xml:space="preserve">`)
		// EscapeText also replaces characters XML can't represent.
		_ = xml.EscapeText(&d.body, []byte(line))
		d.body.WriteString("</w:t>")
	}
	d.body.WriteString("</w:r></w:p>")
}

// Write emits the .docx file.
func (d *Document) Write(w io.Writer) error {
	z := zip.NewWriter(w)
	parts := []struct{ name, content string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"word/_rels/document.xml.rels", documentRels},
		{"word/styles.xml", styles},
		{"word/document.xml", documentHead + d.body.String() + documentTail},
	}
	for _, p := range parts {
		f, err := z.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.content); err != nil {
			return err
		}
	}
	return z.Close()
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const contentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`</Types>`

const rootRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`</Relationships>`

const documentRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`</Relationships>`

const documentHead = xmlHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`

// Letter page with 1" margins.
const documentTail = `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/>` +
	`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/>` +
	`</w:sectPr></w:body></w:document>`

// Sizes are in half-points (22 = 11pt); spacing in twentieths of a point.
const styles = xmlHeader + `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="80"/></w:pPr>` +
	`<w:rPr><w:b/><w:sz w:val="40"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Subtitle"><w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="360"/></w:pPr>` +
	`<w:rPr><w:color w:val="6B675E"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/>` +
	`<w:pPr><w:keepNext/><w:spacing w:before="360" w:after="120"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Note"><w:name w:val="Note"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="80"/></w:pPr>` +
	`<w:rPr><w:color w:val="6B675E"/><w:sz w:val="18"/></w:rPr></w:style>` +
	`</w:styles>`
