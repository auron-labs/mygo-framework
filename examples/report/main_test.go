package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ledongthuc/pdf"
)

func TestReportIsOneValidPageWithTableAndChartText(t *testing.T) {
	data, err := buildReport()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	doc, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	if doc.NumPage() != 1 {
		t.Fatalf("pages = %d, want 1", doc.NumPage())
	}
	reader, err := doc.GetPlainText()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"Weekly sleep report", "Lowest SpO2", "Thu", "93%", "Hours slept", "Page 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q", want)
		}
	}
}
