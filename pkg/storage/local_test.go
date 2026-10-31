package storage_test

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"io"
	"os"
	"sea-codex-server/pkg/storage"
	"testing"
)

func TestPrivateStorageValidationAndCleanup(t *testing.T) {
	dir := t.TempDir()
	s := storage.NewLocal(dir)
	ctx := context.Background()
	var imageData bytes.Buffer
	require.NoError(t, png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	object, err := s.Put(ctx, bytes.NewReader(imageData.Bytes()), 1024)
	require.NoError(t, err)
	assert.NotEmpty(t, object.SHA256)
	assert.Equal(t, "image/png", object.MIME)
	file, err := s.Open(ctx, object.Key)
	require.NoError(t, err)
	actual, err := io.ReadAll(file)
	file.Close()
	require.NoError(t, err)
	assert.Equal(t, imageData.Bytes(), actual)
	_, err = s.Open(ctx, "../../etc/passwd")
	assert.ErrorIs(t, err, storage.ErrInvalidFile)
	_, err = s.Put(ctx, bytes.NewReader([]byte("<script>alert(1)</script>")), 1024)
	assert.ErrorIs(t, err, storage.ErrInvalidFile)
	_, err = s.Put(ctx, bytes.NewReader(imageData.Bytes()), 1)
	assert.ErrorIs(t, err, storage.ErrInvalidFile)
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	require.NoError(t, s.Delete(object.Key))
	files, err = os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, files)
}
