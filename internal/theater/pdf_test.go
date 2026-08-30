package theater

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestExtractPDFTextLayer(t *testing.T) {
	text, err := ExtractPDFText(testPDF("Hello Role"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Hello Role") {
		t.Fatalf("text=%q", text)
	}
}

func TestExtractPDFRejectsMissingText(t *testing.T) {
	if _, err := ExtractPDFText(nil, 4096); err == nil {
		t.Fatal("empty PDF accepted")
	}
}

func testPDF(text string) []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len("BT /F1 12 Tf 72 720 Td ("+text+") Tj ET"), "BT /F1 12 Tf 72 720 Td ("+text+") Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}
