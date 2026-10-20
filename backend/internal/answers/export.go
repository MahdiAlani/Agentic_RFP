package answers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"rfp-agent/internal/docx"
)

const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// buildResponse lays out an RFP response: each question, its answer, the
// sources it cites, and a note on anything not yet approved.
func buildResponse(projectName string, answers []*answer, now time.Time) *docx.Document {
	approved := 0
	for _, a := range answers {
		if a.Status == "approved" {
			approved++
		}
	}

	doc := &docx.Document{}
	doc.Title(projectName)
	doc.Subtitle(fmt.Sprintf("Response prepared %s · %d of %d answers approved",
		now.Format("January 2, 2006"), approved, len(answers)))

	for _, a := range answers {
		doc.Heading(a.Question)

		if a.AnswerText == nil || strings.TrimSpace(*a.AnswerText) == "" {
			doc.Placeholder("No answer yet.")
			continue
		}
		doc.Paragraph(*a.AnswerText)

		if len(a.Citations) > 0 {
			refs := make([]string, len(a.Citations))
			for i, c := range a.Citations {
				refs[i] = fmt.Sprintf("[%d] %s", c.Position, c.FileName)
			}
			doc.Note("Sources: " + strings.Join(refs, "; "))
		}
		if a.Status != "approved" {
			doc.Note("Draft - not yet approved.")
		}
	}
	return doc
}

var unsafeFilename = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// exportFilename turns a project name into a safe download name.
func exportFilename(projectName string) string {
	base := strings.Trim(unsafeFilename.ReplaceAllString(projectName, "_"), "_.")
	if base == "" {
		base = "rfp"
	}
	return base + "-response.docx"
}
