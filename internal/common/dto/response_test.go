package dto_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/common/dto"
)

func TestResponseEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		status  int
		failure bool
	}{
		{"success", http.StatusCreated, false}, {"error", http.StatusBadRequest, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if tc.failure {
				dto.ErrorResponse(c, tc.status, "safe message")
			} else {
				dto.SuccessResponse(c, tc.status, "safe message", gin.H{"id": "example"})
			}
			assert.Equal(t, tc.status, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Len(t, body, 3)
			assert.Equal(t, float64(tc.status), body["code"])
			assert.Equal(t, "safe message", body["message"])
			if tc.failure {
				assert.Nil(t, body["data"])
				assert.True(t, c.IsAborted())
			} else {
				assert.NotNil(t, body["data"])
			}
		})
	}
}
