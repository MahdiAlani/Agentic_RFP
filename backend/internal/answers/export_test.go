package answers

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func documentXML(t *testing.T, as []*answer) string {
	t.Helper()
	var buf bytes.Buffer
	if err := buildResponse("City of Ashford RFP", as, time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)).Write(&buf); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	t.Fatal("no word/document.xml")
	return ""
}

func TestBuildResponse(t *testing.T) {
	approved, draft := "Encrypted with AES-256 [1].", "Hourly backups [1]."
	doc := documentXML(t, []*answer{
		{ID: uuid.New(), Question: "3.1.1 Encryption?", AnswerText: &approved, Status: "approved",
			Citations: []citation{{Position: 1, FileName: "security.md"}}},
		{ID: uuid.New(), Question: "3.2.2 Backups?", AnswerText: &draft, Status: "done",
			Citations: []citation{{Position: 1, FileName: "sla.md"}}},
		{ID: uuid.New(), Question: "3.4.1 WCAG?", Status: "new"},
	})

	for _, want := range []string{
		"City of Ashford RFP",
		"October 18, 2026 · 1 of 3 answers approved",
		"3.1.1 Encryption?", "Encrypted with AES-256 [1].", "Sources: [1] security.md",
		"Sources: [1] sla.md",
		"No answer yet.",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
	// Only the unapproved draft is marked as a draft.
	if n := strings.Count(doc, "Draft - not yet approved."); n != 1 {
		t.Errorf("draft notes = %d, want 1", n)
	}
}

func TestExportFilename(t *testing.T) {
	cases := map[string]string{
		"City of Ashford - Data Analytics RFP": "City_of_Ashford_-_Data_Analytics_RFP-response.docx",
		`../../etc/"passwd"`:                   "etc_passwd-response.docx",
		"   ":                                  "rfp-response.docx",
	}
	for name, want := range cases {
		if got := exportFilename(name); got != want {
			t.Errorf("exportFilename(%q) = %q, want %q", name, got, want)
		}
	}
}
