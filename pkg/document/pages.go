// Package document prepares bounded image/PDF pages without executing a shell.
package document

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"sea-codex-server/pkg/ai"
)

var ErrDocument = errors.New("document cannot be prepared")

type Page struct {
	Number int
	Image  ai.Image
}

func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(data); {
		if data[offset] != 0xff {
			return 1
		}
		marker := data[offset+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		length := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if length < 2 || offset+2+length > len(data) {
			return 1
		}
		segment := data[offset+4 : offset+2+length]
		offset += length + 2
		if marker != 0xe1 || len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
			continue
		}
		tiff := segment[6:]
		var order binary.ByteOrder
		if string(tiff[:2]) == "II" {
			order = binary.LittleEndian
		} else if string(tiff[:2]) == "MM" {
			order = binary.BigEndian
		} else {
			return 1
		}
		start := int(order.Uint32(tiff[4:8]))
		if start < 0 || start+2 > len(tiff) {
			return 1
		}
		count := int(order.Uint16(tiff[start : start+2]))
		for i := 0; i < count; i++ {
			entry := start + 2 + i*12
			if entry+12 > len(tiff) {
				return 1
			}
			if order.Uint16(tiff[entry:entry+2]) == 0x0112 && order.Uint16(tiff[entry+2:entry+4]) == 3 && order.Uint32(tiff[entry+4:entry+8]) == 1 {
				value := int(order.Uint16(tiff[entry+8 : entry+10]))
				if value >= 1 && value <= 8 {
					return value
				}
			}
		}
		return 1
	}
	return 1
}
func normalize(data []byte, max int) (ai.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 20000000 {
		return ai.Image{}, ErrDocument
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ai.Image{}, ErrDocument
	}
	w, h := cfg.Width, cfg.Height
	orientation := exifOrientation(data)
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	outW, outH := ow, oh
	if outW > max || outH > max {
		if outW >= outH {
			outH = outH * max / outW
			outW = max
		} else {
			outW = outW * max / outH
			outH = max
		}
		if outW < 1 {
			outW = 1
		}
		if outH < 1 {
			outH = 1
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for y := 0; y < outH; y++ {
		for x := 0; x < outW; x++ {
			ox, oy := x*ow/outW, y*oh/outH
			sx, sy := ox, oy
			switch orientation {
			case 2:
				sx = w - 1 - ox
			case 3:
				sx = w - 1 - ox
				sy = h - 1 - oy
			case 4:
				sy = h - 1 - oy
			case 5:
				sx = oy
				sy = ox
			case 6:
				sx = oy
				sy = h - 1 - ox
			case 7:
				sx = w - 1 - oy
				sy = h - 1 - ox
			case 8:
				sx = w - 1 - oy
				sy = ox
			}
			c := src.At(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy)
			r, g, b, a := c.RGBA()
			alpha := uint32(0xffff - a)
			dst.SetRGBA(x, y, color.RGBA{R: uint8((r + alpha) >> 8), G: uint8((g + alpha) >> 8), B: uint8((b + alpha) >> 8), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err = jpeg.Encode(&buffer, dst, &jpeg.Options{Quality: 85}); err != nil {
		return ai.Image{}, ErrDocument
	}
	return ai.Image{MIME: "image/jpeg", Data: buffer.Bytes()}, nil
}

type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > b.max {
		return 0, ErrDocument
	}
	return b.Buffer.Write(p)
}
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out := &limitedBuffer{max: 65536}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, ErrDocument
	}
	return out.Bytes(), nil
}

var pagesPattern = regexp.MustCompile(`(?m)^Pages:\s+([0-9]+)`)

func Prepare(ctx context.Context, source *os.File, mime string, maxPages, maxDimension int, work string) ([]Page, error) {
	if mime != "application/pdf" {
		data, err := io.ReadAll(io.LimitReader(source, 100*1024*1024+1))
		if err != nil || len(data) > 100*1024*1024 {
			return nil, ErrDocument
		}
		img, err := normalize(data, maxDimension)
		if err != nil {
			return nil, err
		}
		if err = quality(img.Data, 1); err != nil {
			return nil, err
		}
		return []Page{{Number: 1, Image: img}}, nil
	}
	info, err := run(ctx, "pdfinfo", source.Name())
	if err != nil {
		return nil, err
	}
	match := pagesPattern.FindSubmatch(info)
	if len(match) != 2 {
		return nil, ErrDocument
	}
	count, err := strconv.Atoi(string(match[1]))
	if err != nil || count < 1 || count > maxPages {
		return nil, ErrDocument
	}
	pages := []Page{}
	for i := 1; i <= count; i++ {
		prefix := filepath.Join(work, fmt.Sprintf("page-%d", i))
		if _, err = run(ctx, "pdftoppm", "-f", strconv.Itoa(i), "-l", strconv.Itoa(i), "-singlefile", "-scale-to", strconv.Itoa(maxDimension), "-png", source.Name(), prefix); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(prefix + ".png")
		if err != nil {
			return nil, ErrDocument
		}
		img, err := normalize(data, maxDimension)
		if err != nil {
			return nil, err
		}
		if err = quality(img.Data, i); err != nil {
			return nil, err
		}
		pages = append(pages, Page{Number: i, Image: img})
		_ = os.Remove(prefix + ".png")
	}
	return pages, nil
}
func Split(ctx context.Context, source string, pages []int, work string) (string, error) {
	if len(pages) == 0 {
		return "", ErrDocument
	}
	args := []string{}
	for i, page := range pages {
		pattern := filepath.Join(work, fmt.Sprintf("part-%d-%%d.pdf", i))
		if _, err := run(ctx, "pdfseparate", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), source, pattern); err != nil {
			return "", err
		}
		args = append(args, filepath.Join(work, fmt.Sprintf("part-%d-%d.pdf", i, page)))
	}
	output := filepath.Join(work, "derived.pdf")
	args = append(args, output)
	if _, err := run(ctx, "pdfunite", args...); err != nil {
		return "", err
	}
	return output, nil
}
