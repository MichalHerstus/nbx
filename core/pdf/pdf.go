// Package pdf provides the swappable server-side report-to-PDF renderer used
// by the NextBase reports feature (plan F3).
//
// An interactive dashboard cannot be fully rendered server-side, so the PDF
// renderer consumes a plain, renderer-agnostic document model (title, text,
// key-value metrics and data tables) which the reports API builds from the
// dashboard widget configuration. This keeps the PDF generation independent
// from any particular rendering engine.
package pdf

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

// Metric is a single key/value pair rendered as a report stat (eg. a KPI
// "count" with its numeric result).
type Metric struct {
	Label string
	Value string
}

// Table is a simple row-oriented data table.
type Table struct {
	Title   string
	Columns []string
	Rows    [][]string
}

// Doc is the renderer-agnostic report model.
type Doc struct {
	Title    string
	Subtitle string

	// Stats are rendered as a summary band (typically KPI widgets).
	Stats []Metric

	// Tables hold the report page tables (from "table" widgets).
	Tables []Table

	// Notes are free text paragraphs (from "text" widgets).
	Notes []string
}

// Renderer is the swappable report-to-PDF contract.
type Renderer interface {
	// Render reports the provided document as PDF bytes.
	Render(d Doc) ([]byte, error)

	// Mime returns the renderer output MIME type (usually "application/pdf").
	Mime() string
}

// -------------------------------------------------------------------

const defaultFont = "Helvetica"

// FpdfRenderer is a pure-Go [Renderer] backed by go-pdf/fpdf.
type FpdfRenderer struct{}

// Mime implements [Renderer.Mime].
func (r *FpdfRenderer) Mime() string {
	return "application/pdf"
}

// Render implements [Renderer.Render].
func (r *FpdfRenderer) Render(d Doc) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	margin := 12.0
	width := 210.0 - 2*margin // A4 portrait content width

	// header
	pdf.SetFont(defaultFont, "B", 16)
	pdf.SetTextColor(20, 20, 30)
	pdf.Cell(0, 10, truncate(d.Title, 90))
	pdf.Ln(11)
	if d.Subtitle != "" {
		pdf.SetFont(defaultFont, "I", 10)
		pdf.SetTextColor(110, 110, 120)
		pdf.SetX(margin)
		pdf.Cell(0, 6, truncate(d.Subtitle, 140))
		pdf.Ln(8)
	}

	// stats band
	if len(d.Stats) > 0 {
		pdf.SetFillColor(238, 240, 244)
		cellW := width / float64(len(d.Stats))
		for i, m := range d.Stats {
			x := margin + float64(i)*cellW
			pdf.SetX(x)
			pdf.Rect(x, pdf.GetY(), cellW-2, 20, "F")
			pdf.SetFont(defaultFont, "B", 13)
			pdf.SetTextColor(30, 30, 40)
			pdf.SetXY(x+3, pdf.GetY()+2)
			pdf.Cell(cellW-6, 8, truncate(m.Value, 18))
			pdf.SetFont(defaultFont, "", 8)
			pdf.SetTextColor(120, 120, 130)
			pdf.SetX(x + 3)
			pdf.Cell(cellW-6, 6, truncate(m.Label, 24))
		}
		pdf.SetY(pdf.GetY() + 24)
	}

	// notes
	for _, note := range d.Notes {
		ensureSpace(pdf, 20)
		pdf.SetFont(defaultFont, "", 9)
		pdf.SetTextColor(50, 50, 60)
		pdf.MultiCell(width, 5, note, "", "L", false)
		pdf.Ln(4)
	}

	// tables
	for _, table := range d.Tables {
		renderTable(pdf, table, width, margin)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// ensureSpace adds a page break when less than minH mm remain on the page.
func ensureSpace(pdf *fpdf.Fpdf, minH float64) {
	_, pageH := pdf.GetPageSize()
	if pdf.GetY()+minH >= pageH-15 {
		pdf.AddPage()
	}
}

// renderTable draws a table block with a heading, header row and body rows.
func renderTable(pdf *fpdf.Fpdf, table Table, width, margin float64) {
	if table.Title != "" {
		ensureSpace(pdf, 16)
		pdf.SetFont(defaultFont, "B", 12)
		pdf.SetTextColor(30, 30, 40)
		pdf.SetX(margin)
		pdf.Cell(width, 8, truncate(table.Title, 70))
		pdf.Ln(9)
	}

	cols := len(table.Columns)
	if cols == 0 {
		cols = 1
	}
	colW := width / float64(cols)

	drawRow := func(cells []string, fill bool, isHeader bool) {
		maxH := 6.0
		for i := 0; i < cols; i++ {
			if i < len(cells) {
				h := float64(len(pdf.SplitLines([]byte(cells[i]), colW))) * 4.5
				if h > maxH {
					maxH = h
				}
			}
		}
		cellH := maxH + 2

		if fill {
			pdf.SetFillColor(242, 243, 246)
			pdf.Rect(margin, pdf.GetY(), width, cellH, "F")
		}
		pdf.SetFont(defaultFont, "B", 8.5)
		pdf.SetTextColor(50, 50, 60)
		for i := 0; i < cols; i++ {
			x := margin + float64(i)*colW
			pdf.SetXY(x+2, pdf.GetY()+1)
			if i < len(cells) {
				pdf.MultiCell(colW-3, 4.5, cells[i], "", "L", false)
			}
		}
		pdf.SetFont(defaultFont, "", 8.5)
		pdf.SetTextColor(40, 40, 50)
		pdf.Ln(cellH + 1)
	}

	for _, c := range table.Columns {
		drawRow([]string{c}, true, true)
	}
	for _, row := range table.Rows {
		drawRow(row, false, false)
	}
	pdf.Ln(3)
}

// truncate limits s to at most maxR runes (ellipsizing the tail).
func truncate(s string, maxR int) string {
	if maxR <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxR {
		return s
	}
	return string(runes[:maxR]) + "…"
}