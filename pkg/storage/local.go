// Package storage manages opaque private object keys, never client supplied paths.
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
)

var ErrInvalidFile = errors.New("invalid or oversized file")

type Object struct {
	Key    string
	SHA256 string
	MIME   string
	Size   int64
}
type Local struct{ directory string }

func NewLocal(directory string) *Local { return &Local{directory: directory} }

var keyPattern = regexp.MustCompile(`^[0-9a-f-]{36}\.(pdf|png|jpg)$`)

func (s *Local) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", ErrInvalidFile
	}
	return filepath.Join(s.directory, key), nil
}
func (s *Local) Put(ctx context.Context, reader io.Reader, max int64) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	prefix := make([]byte, 512)
	n, err := io.ReadFull(reader, prefix)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return Object{}, err
	}
	prefix = prefix[:n]
	mime := http.DetectContentType(prefix)
	extension := ""
	switch mime {
	case "application/pdf":
		extension = "pdf"
	case "image/png":
		extension = "png"
	case "image/jpeg":
		extension = "jpg"
	default:
		return Object{}, ErrInvalidFile
	}
	if err = os.MkdirAll(s.directory, 0750); err != nil {
		return Object{}, err
	}
	key := uuid.NewString() + "." + extension
	path, _ := s.path(key)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return Object{}, err
	}
	good := false
	defer func() {
		file.Close()
		if !good {
			_ = os.Remove(path)
		}
	}()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(io.MultiReader(bytes.NewReader(prefix), reader), max+1))
	if err != nil {
		return Object{}, err
	}
	if count > max || count == 0 {
		return Object{}, ErrInvalidFile
	}
	if err = ctx.Err(); err != nil {
		return Object{}, err
	}
	if err = file.Sync(); err != nil {
		return Object{}, err
	}
	if err = file.Close(); err != nil {
		return Object{}, err
	}
	good = true
	return Object{Key: key, SHA256: hex.EncodeToString(hash.Sum(nil)), MIME: mime, Size: count}, nil
}

func (s *Local) Open(ctx context.Context, key string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *Local) Delete(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
