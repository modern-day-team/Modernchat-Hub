package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIPRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := NewIPRateLimiter(3) // 每 IP 3 次/分
	r := gin.New()
	r.POST("/x", l.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 0, 5)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK || codes[2] != http.StatusOK {
		t.Fatalf("突发 3 次应全部放行: %v", codes)
	}
	if codes[3] != http.StatusTooManyRequests || codes[4] != http.StatusTooManyRequests {
		t.Fatalf("超限后应 429: %v", codes)
	}
	// 429 响应体为项目组统一文案
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	r.ServeHTTP(w, req)
	if w.Body.String() == "" || w.Body.String()[0:1] != "{" {
		t.Fatalf("429 应带 JSON 错误体: %q", w.Body.String())
	}
}

func TestIPRateLimiterPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := NewIPRateLimiter(1)
	r := gin.New()
	r.POST("/x", l.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })

	// A 用尽配额
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "1.1.1.1:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("A 第一次应放行: %d", w.Code)
	}
	// B 不受影响
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "2.2.2.2:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("B 的配额应独立: %d", w.Code)
	}
	// A 再来 → 429
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "1.1.1.1:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("A 超限应 429: %d", w.Code)
	}
}
