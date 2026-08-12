package pdf_test

import (
	"bytes"
	"testing"

	"github.com/pocketbase/pocketbase/core/pdf"
)

func TestFpdfRenderer_Render(t *testing.T) {
	r := &pdf.FpdfRenderer{}

	doc := pdf.Doc{
		Title:    "Sales report",
		Subtitle: "Quarterly summary",
		Stats: []pdf.Metric{
			{Label: "Orders", Value: "124"},
			{Label: "Revenue", Value: "9,430"},
		},
		Notes: []string{"This is a generated report."},
		Tables: []pdf.Table{
			{
				Title:   "Top products",
				Columns: []string{"Product", "Qty"},
				Rows: [][]string{
					{"Widget", "12"},
					{"Gadget", "7"},
				},
			},
		},
	}

	b, err := r.Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mime() != "application/pdf" {
		t.Fatalf("unexpected mime %q", r.Mime())
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected PDF magic header, got %q", b[:5])
	}
}

func TestFpdfRenderer_Empty(t *testing.T) {
	r := &pdf.FpdfRenderer{}
	b, err := r.Render(pdf.Doc{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected PDF magic header")
	}
}

func TestFpdfRenderer_LongText(t *testing.T) {
	r := &pdf.FpdfRenderer{}
	long := ""
	for i := 0; i < 300; i++ {
		long += "word "
	}
	if _, err := r.Render(pdf.Doc{Notes: []string{long}}); err != nil {
		t.Fatalf("expected long text to render without error: %v", err)
	}
}