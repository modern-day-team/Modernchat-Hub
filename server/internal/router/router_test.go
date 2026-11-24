package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/config"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
)

// setupTestRouter 不传真实 DB：Setup 只构造 store/service，公开端点不查库，
// 受保护端点在到达 handler 前就被 Auth 拦截。
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		JWTSecret:        "test-jwt-secret-0123456789abcdef-0123456789abcdef",
		TrustedProxies:   []string{"127.0.0.1", "::1"},
		LoginMaxAttempts: 5,
		LoginLockMinutes: 15,
	}
	return Setup(cfg, nil, nil, nil, nil, crypto.NewBox("", cfg.JWTSecret))
}

func request(r *gin.Engine, method, path string, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// TestPublicEndpoints 健康检查与版本免鉴权。
func TestPublicEndpoints(t *testing.T) {
	r := setupTestRouter()
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/healthz", `"status":"ok"`},
		{"/api/version", `"version"`},
	} {
		w := request(r, http.MethodGet, tc.path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s 应 200，got %d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("GET %s 响应缺 %s: %s", tc.path, tc.want, w.Body.String())
		}
	}
}

// TestOAuthTokenEndpointIsPublic token 端点必须公开（客户端无 Hub 登录态）：
// 空 body 在鉴权之前就被参数绑定拦为 400，而不是 401——证明该端点没有挂 Auth。
func TestOAuthTokenEndpointIsPublic(t *testing.T) {
	w := request(setupTestRouter(), http.MethodPost, "/api/oauth/token", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺参数应 400（而非 401），got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("400 响应应含 error 字段，got %s", w.Body.String())
	}
}

// TestProtectedEndpointsRequireAuth 用户端点与站长端点无凭证必须 401。
func TestProtectedEndpointsRequireAuth(t *testing.T) {
	r := setupTestRouter()
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users/me"},
		{http.MethodDelete, "/api/v1/sessions/current"},
		{http.MethodGet, "/api/v1/services"},
		{http.MethodPost, "/api/v1/services"},
		{http.MethodGet, "/api/v1/services/1"},
		{http.MethodPost, "/api/v1/services/1/actions/check"},
		{http.MethodGet, "/api/v1/services/1/admin/users"},
		{http.MethodGet, "/oauth/authorize"},
	} {
		w := request(r, tc.method, tc.path, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s 应 401，got %d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// TestNotFoundRoute 未注册路径必须 404。
func TestNotFoundRoute(t *testing.T) {
	w := request(setupTestRouter(), http.MethodGet, "/api/v1/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("未知路径应 404，got %d", w.Code)
	}
}
