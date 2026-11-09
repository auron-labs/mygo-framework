package pdfreport

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/ledongthuc/pdf"
)

func read(t *testing.T, r *Report) (pages int, text string) {
	t.Helper()
	data, err := r.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	plain, err := doc.GetPlainText()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(plain)
	if err != nil {
		t.Fatal(err)
	}
	return doc.NumPage(), string(raw)
}

func TestLongTableContinuesWithRepeatedHeaderAndPageNumbers(t *testing.T) {
	r, err := New(Options{Title: "Long", Footer: "Footer text"})
	if err != nil {
		t.Fatal(err)
	}
	rows := make([][]string, 120)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("row-%03d", i), strconv.Itoa(i)}
	}
	r.Table([]Column{{Title: "Name", Width: 2}, {Title: "Value", Width: 1, Right: true}}, rows)
	pages, text := read(t, r)
	if pages < 2 {
		t.Fatalf("pages = %d, want the table to span several", pages)
	}
	if got := strings.Count(text, "Name"); got != pages {
		t.Errorf("header repeated %d times over %d pages", got, pages)
	}
	for _, want := range []string{"row-000", "row-119", "Footer text", fmt.Sprintf("Page %d", pages)} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q", want)
		}
	}
}

func TestParagraphWrapsAndChartDrawsLabels(t *testing.T) {
	r, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	r.Paragraph(strings.Repeat("wrapped words ", 80))
	r.LineChart(LineChart{
		Title:  "Trend",
		Series: []Series{{Name: "Flat", Points: []Point{{0, 5}, {1, 5}}}},
		YLabel: func(v float64) string { return fmt.Sprintf("%g units", v) },
	})
	pages, text := read(t, r)
	if pages != 1 {
		t.Fatalf("pages = %d", pages)
	}
	for _, want := range []string{"Trend", "Flat", "units"} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q", want)
		}
	}
}

func TestInvalidContentIsReported(t *testing.T) {
	cases := map[string]func(*Report){
		"no columns":  func(r *Report) { r.Table(nil, nil) },
		"ragged row":  func(r *Report) { r.Table([]Column{{Title: "A"}, {Title: "B"}}, [][]string{{"only one"}}) },
		"empty chart": func(r *Report) { r.LineChart(LineChart{Series: []Series{{Name: "none"}}}) },
		"non-finite data": func(r *Report) {
			r.LineChart(LineChart{Series: []Series{{Name: "bad", Points: []Point{{0, 1}, {1, nan()}}}}})
		},
	}
	for name, add := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := New(Options{})
			if err != nil {
				t.Fatal(err)
			}
			add(r)
			if _, err := r.Bytes(); err == nil {
				t.Fatal("invalid content produced a PDF")
			}
		})
	}
}

func nan() float64 { var z float64; return z / z }
