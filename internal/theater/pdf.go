package theater

import (
	"fmt"
	"io"
	"os"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

func ExtractPDFText(raw []byte, maxOutput int64) (text string, err error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("empty PDF")
	}
	if maxOutput <= 0 {
		maxOutput = 4 << 20
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			text = ""
			err = fmt.Errorf("PDF text extraction failed")
		}
	}()
	file, err := os.CreateTemp("", "deep-seeing-role-*.pdf")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return "", err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	handle, reader, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer handle.Close()
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(plain, maxOutput+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > maxOutput {
		return "", fmt.Errorf("extracted PDF text exceeds limit")
	}
	text = strings.TrimSpace(string(body))
	if text == "" {
		return "", fmt.Errorf("PDF has no text layer; OCR is not supported yet")
	}
	return text, nil
}
