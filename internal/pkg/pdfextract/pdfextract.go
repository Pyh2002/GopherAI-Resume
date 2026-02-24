package pdfextract

import (
	"bytes"
	"io"

	"github.com/ledongthuc/pdf"
)

// ExtractText reads the entire content of r and extracts plain text from the PDF.
// Returns empty string and nil error if the PDF has no extractable text.
func ExtractText(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", nil
	}
	readerAt := bytes.NewReader(b)
	pdfReader, err := pdf.NewReader(readerAt, int64(len(b)))
	if err != nil {
		return "", err
	}
	plainReader, err := pdfReader.GetPlainText()
	if err != nil {
		return "", err
	}
	out, err := io.ReadAll(plainReader)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
