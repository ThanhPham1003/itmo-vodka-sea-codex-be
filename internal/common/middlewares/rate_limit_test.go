package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clock := time.Now()
	l := NewRateLimiter(1, time.Minute)
	l.now = func() time.Time { return clock }
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.GET("/test", l.Middleware(), func(c *gin.Context) { c.Status(204) })
	for _, status := range []int{204, 429} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		r.ServeHTTP(w, req)
		assert.Equal(t, status, w.Code)
	}
	clock = clock.Add(time.Minute)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	assert.Equal(t, 204, w.Code)
}
