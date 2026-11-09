// Package pdfreport writes small business-style PDF reports: headings, text,
// tables and line charts under a page header and footer. It wraps
// github.com/signintech/gopdf, which is pure Go, so reports build without CGO
// on every platform.
//
// Native MyGo windows cannot be printed to PDF (Page.PrintToPDF needs a web
// view), so a native app lays its report out with this package instead.
package pdfreport

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	regular = "regular"
	bold    = "bold"

	margin     = 48.0
	bodySize   = 10.0
	lineHeight = 15.0
	rowHeight  = 20.0
)

// Options configures a Report. The zero value is a headerless, footerless A4
// portrait report.
type Options struct {
	// Title is stored in the document properties and shown in the page header.
	Title string
	// Footer is shown at the bottom of every page beside the page number.
	Footer string
}

// Report accumulates content and renders it with Bytes. Content that does not
// fit on a page continues on the next one. A Report is not safe for concurrent
// use.
type Report struct {
	pdf  *gopdf.GoPdf
	opts Options
	page int
	err  error
}

// New starts a report with its first page.
func New(opts Options) (*Report, error) {
	r := &Report{pdf: &gopdf.GoPdf{}, opts: opts}
	r.pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	r.pdf.SetMargins(margin, margin, margin, margin)
	if err := r.pdf.AddTTFFontData(regular, goregular.TTF); err != nil {
		return nil, fmt.Errorf("load regular font: %w", err)
	}
	if err := r.pdf.AddTTFFontData(bold, gobold.TTF); err != nil {
		return nil, fmt.Errorf("load bold font: %w", err)
	}
	r.pdf.SetInfo(gopdf.PdfInfo{Title: opts.Title, Producer: "pdfreport"})
	r.pdf.AddHeader(r.drawHeader)
	r.pdf.AddFooter(r.drawFooter)
	r.pdf.AddPage()
	return r, r.err
}

func (r *Report) drawHeader() {
	r.page++
	if r.opts.Title == "" {
		return
	}
	r.setFont(regular, 9)
	r.pdf.SetTextColor(110, 110, 110)
	r.pdf.SetXY(margin, margin/2)
	r.check(r.pdf.Text(r.opts.Title))
	r.pdf.SetStrokeColor(200, 200, 200)
	r.pdf.SetLineWidth(0.5)
	r.pdf.Line(margin, margin-8, r.contentRight(), margin-8)
}

func (r *Report) drawFooter() {
	y := gopdf.PageSizeA4.H - margin/2
	r.setFont(regular, 9)
	r.pdf.SetTextColor(110, 110, 110)
	r.pdf.SetXY(margin, y)
	r.check(r.pdf.Text(r.opts.Footer))
	label := fmt.Sprintf("Page %d", r.page)
	width, err := r.pdf.MeasureTextWidth(label)
	r.check(err)
	r.pdf.SetXY(r.contentRight()-width, y)
	r.check(r.pdf.Text(label))
}

func (r *Report) contentRight() float64 { return gopdf.PageSizeA4.W - margin }
func (r *Report) contentWidth() float64 { return gopdf.PageSizeA4.W - 2*margin }
func (r *Report) bottom() float64       { return gopdf.PageSizeA4.H - margin }

// check records the first drawing error so call sites stay linear; Bytes
// reports it.
func (r *Report) check(err error) {
	if err != nil && r.err == nil {
		r.err = err
	}
}

func (r *Report) setFont(family string, size float64) {
	r.check(r.pdf.SetFont(family, "", size))
}

// ensure starts a new page unless height points still fit on this one.
func (r *Report) ensure(height float64) {
	if r.pdf.GetY()+height > r.bottom() {
		r.pdf.AddPage()
		r.pdf.SetY(margin)
	}
}

// Heading adds a bold line of text above the content that follows.
func (r *Report) Heading(text string, size float64) {
	r.ensure(size * 2.5)
	r.setFont(bold, size)
	r.pdf.SetTextColor(20, 20, 20)
	// Text positions the baseline, so move down by the font size first.
	r.pdf.SetXY(margin, r.pdf.GetY()+size*0.5+size)
	r.check(r.pdf.Text(text))
	r.pdf.SetY(r.pdf.GetY() + size*0.6)
}

// Paragraph adds body text wrapped to the page width.
func (r *Report) Paragraph(text string) {
	r.setFont(regular, bodySize)
	r.pdf.SetTextColor(40, 40, 40)
	lines, err := r.pdf.SplitTextWithWordWrap(text, r.contentWidth())
	r.check(err)
	for _, line := range lines {
		r.ensure(lineHeight)
		r.pdf.SetXY(margin, r.pdf.GetY()+bodySize)
		r.check(r.pdf.Text(line))
		r.pdf.SetY(r.pdf.GetY() + lineHeight - bodySize)
	}
	r.pdf.SetY(r.pdf.GetY() + lineHeight/2)
}

// Column is one table column. Width is its share of the page width relative to
// the other columns.
type Column struct {
	Title string
	Width float64
	// Right aligns the column's cells, as numbers read best.
	Right bool
}

// Table adds a table whose header row repeats on every page it spans. Each row
// must have one cell per column.
func (r *Report) Table(columns []Column, rows [][]string) {
	if len(columns) == 0 {
		r.check(errors.New("table needs at least one column"))
		return
	}
	for i, row := range rows {
		if len(row) != len(columns) {
			r.check(fmt.Errorf("table row %d has %d cells for %d columns", i, len(row), len(columns)))
			return
		}
	}
	widths := columnWidths(columns, r.contentWidth())
	r.ensure(2 * rowHeight)
	r.tableRow(columns, widths, titles(columns), true)
	for _, row := range rows {
		if r.pdf.GetY()+rowHeight > r.bottom() {
			r.pdf.AddPage()
			r.pdf.SetY(margin)
			r.tableRow(columns, widths, titles(columns), true)
		}
		r.tableRow(columns, widths, row, false)
	}
	r.pdf.SetY(r.pdf.GetY() + lineHeight/2)
}

func titles(columns []Column) []string {
	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = c.Title
	}
	return out
}

func columnWidths(columns []Column, total float64) []float64 {
	var sum float64
	for _, c := range columns {
		sum += c.Width
	}
	widths := make([]float64, len(columns))
	for i, c := range columns {
		if sum == 0 {
			widths[i] = total / float64(len(columns))
			continue
		}
		widths[i] = total * c.Width / sum
	}
	return widths
}

func (r *Report) tableRow(columns []Column, widths []float64, cells []string, header bool) {
	y := r.pdf.GetY()
	if header {
		r.pdf.SetFillColor(238, 240, 244)
		r.pdf.RectFromUpperLeftWithStyle(margin, y, r.contentWidth(), rowHeight, "F")
		r.setFont(bold, bodySize)
	} else {
		r.setFont(regular, bodySize)
	}
	r.pdf.SetTextColor(30, 30, 30)
	x := margin
	for i, cell := range cells {
		const pad = 6.0
		textX := x + pad
		if columns[i].Right {
			w, err := r.pdf.MeasureTextWidth(cell)
			r.check(err)
			textX = x + widths[i] - pad - w
		}
		r.pdf.SetXY(textX, y+14)
		r.check(r.pdf.Text(cell))
		x += widths[i]
	}
	r.pdf.SetStrokeColor(215, 215, 215)
	r.pdf.SetLineWidth(0.5)
	r.pdf.Line(margin, y+rowHeight, r.contentRight(), y+rowHeight)
	r.pdf.SetXY(margin, y+rowHeight)
}

// Bytes finishes the report and returns the PDF document.
func (r *Report) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, fmt.Errorf("build report: %w", r.err)
	}
	var buf bytes.Buffer
	if _, err := r.pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write report: %w", err)
	}
	return buf.Bytes(), nil
}
