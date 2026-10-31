package document

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func syntheticPDF() []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 7 0 R >> >> /Contents 5 0 R >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 7 0 R >> >> /Contents 6 0 R >>"}
	for _, label := range []string{"SYNTHETIC IDENTITY - TEST ONLY", "SYNTHETIC CONTRACT - TEST ONLY"} {
		content := "BT /F1 12 Tf 20 350 Td (" + label + ") Tj ET"
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}
func TestPreparePDFAndPageLimit(t *testing.T) {
	for _, name := range []string{"pdfinfo", "pdftoppm"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("Poppler required for PDF integration")
		}
	}
	work := t.TempDir()
	path := filepath.Join(work, "synthetic.pdf")
	require.NoError(t, os.WriteFile(path, syntheticPDF(), 0600))
	source, err := os.Open(path)
	require.NoError(t, err)
	defer source.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = Prepare(ctx, source, "application/pdf", 1, 600, work)
	assert.ErrorIs(t, err, ErrDocument)
	pages, err := Prepare(ctx, source, "application/pdf", 2, 600, work)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	assert.Equal(t, 2, pages[1].Number)
	assert.Equal(t, "image/jpeg", pages[0].Image.MIME)
	for _, name := range []string{"pdfseparate", "pdfunite"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Log("Split integration requires complete Poppler; CI installs poppler-utils")
			return
		}
	}
	derived, err := Split(ctx, path, []int{2}, work)
	require.NoError(t, err)
	info, err := run(ctx, "pdfinfo", derived)
	require.NoError(t, err)
	match := pagesPattern.FindSubmatch(info)
	require.Len(t, match, 2)
	assert.Equal(t, "1", string(match[1]))
}
func TestNormalizeImageBounds(t *testing.T) {
	var input bytes.Buffer
	require.NoError(t, jpeg.Encode(&input, image.NewRGBA(image.Rect(0, 0, 100, 50)), nil))
	normalized, err := normalize(input.Bytes(), 40)
	require.NoError(t, err)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(normalized.Data))
	require.NoError(t, err)
	assert.Equal(t, 40, cfg.Width)
	assert.Equal(t, 20, cfg.Height)
	_, err = normalize([]byte("not an image"), 40)
	assert.ErrorIs(t, err, ErrDocument)
}

func TestQualityRejectsBlankAndAcceptsTextEdges(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 200, 200))
	var raw bytes.Buffer
	require.NoError(t, jpeg.Encode(&raw, blank, nil))
	var qualityErr *QualityError
	require.ErrorAs(t, quality(raw.Bytes(), 3), &qualityErr)
	assert.Equal(t, 3, qualityErr.Page)
	assert.Equal(t, "BLANK_OR_LOW_CONTRAST_PAGE", qualityErr.Code)
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			if y%20 < 4 {
				blank.Pix[y*blank.Stride+x] = 255
			}
		}
	}
	raw.Reset()
	require.NoError(t, jpeg.Encode(&raw, blank, nil))
	assert.NoError(t, quality(raw.Bytes(), 1))
}
