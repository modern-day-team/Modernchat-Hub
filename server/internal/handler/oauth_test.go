package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/service"
)

// fakeOAuthManage 窄接口假实现，驱动 handler 各分支。
type fakeOAuthManage struct {
	code      string
	token     string
	info      *service.UserInfoView
	err       error
	gotUserID uint
	gotState  string
}

func (f *fakeOAuthManage) RegisterClient(string, string, string) (string, error) {
	return "secret", f.err
}
func (f *fakeOAuthManage) Authorize(_, _, _ string, uid uint) (string, error) {
	f.gotUserID = uid
	return f.code, f.err
}
func (f *fakeOAuthManage) Exchange(_, _, _, _ string) (string, error) {
	return f.token, f.err
}
func (f *fakeOAuthManage) UserInfo(_ string) (*service.UserInfoView, error) {
	return f.info, f.err
}

func newOAuthTestRouter(f *fakeOAuthManage, withUser bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withUser {
		r.Use(func(c *gin.Context) {
			c.Set("user", &model.User{ID: 9, Role: "admin"})
			c.Next()
		})
	}
	h := NewOAuthHandler(f)
	r.GET("/oauth/authorize", h.Authorize)
	r.POST("/api/oauth/token", h.Token)
	r.GET("/api/oauth/userinfo", h.UserInfo)
	return r
}

// TestOAuthAuthorizeRequiresLogin 未登录必须 401，不能发放授权码。
func TestOAuthAuthorizeRequiresLogin(t *testing.T) {
	f := &fakeOAuthManage{code: "code"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=c&redirect_uri=https%3A%2F%2Fexample.com%2Fcb&state=s", nil)
	newOAuthTestRouter(f, false).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，got %d", w.Code)
	}
	if f.gotUserID != 0 {
		t.Fatal("未登录不应调用 Authorize")
	}
}

// TestOAuthAuthorizeEchoesState state 必须原样回传（登录 CSRF 绑定由客户端负责）。
func TestOAuthAuthorizeEchoesState(t *testing.T) {
	f := &fakeOAuthManage{code: "authcode"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=c&redirect_uri=https%3A%2F%2Fexample.com%2Fcb&state=abc123&scope=profile", nil)
	newOAuthTestRouter(f, true).ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("应 302，got %d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "code=authcode") || !strings.Contains(loc, "state=abc123") {
		t.Fatalf("回调应带 code 与原样 state，got %q", loc)
	}
	if f.gotUserID != 9 {
		t.Fatalf("应使用当前用户 ID，got %d", f.gotUserID)
	}
}

// TestOAuthAuthorizeRequiresParams 缺 state 必须 400。
func TestOAuthAuthorizeRequiresParams(t *testing.T) {
	f := &fakeOAuthManage{code: "code"}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=c&redirect_uri=https%3A%2F%2Fexample.com%2Fcb", nil)
	newOAuthTestRouter(f, true).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 state 应 400，got %d", w.Code)
	}
}

// TestOAuthTokenErrorIsJSON token 端点错误也必须是可解析 JSON（客户端只读 error 字段）。
func TestOAuthTokenErrorIsJSON(t *testing.T) {
	f := &fakeOAuthManage{err: apperr.ErrOAuthCode}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/token",
		strings.NewReader(`{"client_id":"c","client_secret":"s","code":"bad","redirect_uri":"https://example.com/cb"}`))
	req.Header.Set("Content-Type", "application/json")
	newOAuthTestRouter(f, true).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("token 错误应 200 带 error，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("响应应含 error 字段，got %s", w.Body.String())
	}
}

// TestOAuthUserInfoRequiresBearer 缺 Bearer 令牌必须返回 error JSON。
func TestOAuthUserInfoRequiresBearer(t *testing.T) {
	f := &fakeOAuthManage{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/userinfo", nil)
	newOAuthTestRouter(f, true).ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("缺令牌应返回 error，got %s", w.Body.String())
	}
}
