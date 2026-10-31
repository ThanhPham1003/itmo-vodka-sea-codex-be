package document

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
)

// QualityError deliberately carries no source text or file path.
type QualityError struct {
	Page int
	Code string
}

func (e *QualityError) Error() string { return fmt.Sprintf("page %d: %s", e.Page, e.Code) }
func (e *QualityError) Unwrap() error { return ErrDocument }

// quality checks normalized pixels with conservative heuristics. It is not an OCR accuracy score.
func quality(data []byte, page int) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ErrDocument
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	gray := make([]float64, w*h)
	var total, squared float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := float64(color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y)
			gray[y*w+x] = v
			total += v
			squared += v * v
		}
	}
	n := float64(w * h)
	variance := math.Max(0, squared/n-(total/n)*(total/n))
	if variance < 1 {
		return &QualityError{Page: page, Code: "BLANK_OR_LOW_CONTRAST_PAGE"}
	}
	// A nearly zero Laplacian on a nonblank page indicates severe loss of detail.
	var edges, edgeSquares, count float64
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			v := 4*gray[i] - gray[i-1] - gray[i+1] - gray[i-w] - gray[i+w]
			edges += v
			edgeSquares += v * v
			count++
		}
	}
	if count > 0 && edgeSquares/count-(edges/count)*(edges/count) < 0.5 {
		return &QualityError{Page: page, Code: "SEVERELY_BLURRED_PAGE"}
	}
	return nil
}
