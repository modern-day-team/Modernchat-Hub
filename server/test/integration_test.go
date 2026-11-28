//go:build integration

// 集成测试：需要本机 MySQL（DB_DSN 环境变量，默认 root:root@tcp(127.0.0.1:3306)/modernchat_hub_test）。
// 运行：go test -tags integration ./test/ -count=1
// 覆盖 OAuth Provider 全链路（真实存储层）：注册首个 admin → 登录 → 注册客户端 →
// authorize 发码 → token 换令牌 → userinfo，以及授权码一次性（重放被拒）。
package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/config"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/router"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/store"
)

const dsnDefault = "root:root@tcp(127.0.0.1:3306)/modernchat_hub_test?charset=utf8mb4&parseTime=true&loc=Local"

func setup(t *testing.T) *httptest.Server {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = dsnDefault
	}
	db, err := store.Open(dsn, true)
	if err != nil {
		t.Skipf("需要本机 MySQL：%v", err)
	}
	// 清理本轮测试数据，保证可重复执行
	for _, tbl := range []string{"o_auth_tokens", "o_auth_codes", "o_auth_clients", "login_attempts", "users"} {
		if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
			t.Fatalf("清理 %s 失败：%v", tbl, err)
		}
	}
	cfg := config.Config{
		JWTSecret:        "integration-test-secret-0123456789abcdef-0123456789abcdef",
		TrustedProxies:   []string{"127.0.0.1"},
		LoginMaxAttempts: 5,
		LoginLockMinutes: 15,
	}
	r := router.Setup(cfg,
		store.NewUserStore(db),
		store.NewInstanceStore(db),
		store.NewLoginAttemptStore(db),
		store.NewOAuthStore(db),
		crypto.NewBox("", cfg.JWTSecret),
	)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, method, path, token, body string) (int, []byte) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s：%v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读响应：%v", err)
	}
	return resp.StatusCode, data
}

// TestOAuthFlow 真实 MySQL 上的 OAuth 全链路 + 授权码重放防护。
func TestOAuthFlow(t *testing.T) {
	ts := setup(t)

	// 1. 注册（首个注册者自动 admin）+ 登录拿 token
	if code, body := do(t, http.MethodPost, ts.URL+"/api/v1/users", "",
		`{"username":"oauth_admin","password":"secret-123"}`); code != http.StatusCreated {
		t.Fatalf("注册应 201，got %d：%s", code, body)
	}
	code, body := do(t, http.MethodPost, ts.URL+"/api/v1/sessions", "",
		`{"username":"oauth_admin","password":"secret-123"}`)
	if code != http.StatusOK {
		t.Fatalf("登录应 200，got %d：%s", code, body)
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &login); err != nil || login.Token == "" {
		t.Fatalf("登录响应缺 token：%s", body)
	}
	adminToken := login.Token

	// 2. 注册 OAuth 客户端（admin 专用端点，返回一次性明文 secret）
	code, body = do(t, http.MethodPost, ts.URL+"/api/v1/oauth/clients", adminToken,
		`{"clientId":"modernchat-new-ci","redirectUri":"https://app.example.com/auth/callback","scopes":"profile"}`)
	if code != http.StatusCreated {
		t.Fatalf("注册客户端应 201，got %d：%s", code, body)
	}
	var client struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.Unmarshal(body, &client); err != nil || client.ClientSecret == "" {
		t.Fatalf("注册客户端响应缺 clientSecret：%s", body)
	}

	// 3. authorize 发码：302 重定向携带 code + 原样回传的 state。
	// 客户端禁止跟随重定向（Location 指向外部 redirect_uri，真实网络不可达）。
	authzURL := ts.URL + "/oauth/authorize?client_id=" + client.ClientID +
		"&redirect_uri=" + url.QueryEscape("https://app.example.com/auth/callback") +
		"&state=st-123&scope=profile"
	req, _ := http.NewRequest(http.MethodGet, authzURL, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatalf("authorize：%v", err)
	}
	loc := resp.Header.Get("Location")
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || loc == "" {
		t.Fatalf("authorize 应 302，got %d", resp.StatusCode)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location 解析失败：%v", err)
	}
	authCode := u.Query().Get("code")
	state := u.Query().Get("state")
	if authCode == "" {
		t.Fatalf("重定向缺 code：%s", loc)
	}
	if state != "st-123" {
		t.Fatalf("state 应原样回传，got %q", state)
	}

	// 4. token 换令牌
	code, body = do(t, http.MethodPost, ts.URL+"/api/oauth/token", "",
		`{"client_id":"`+client.ClientID+`","client_secret":"`+client.ClientSecret+
			`","code":"`+authCode+`","redirect_uri":"https://app.example.com/auth/callback"}`)
	if code != http.StatusOK || strings.Contains(string(body), `"error"`) {
		t.Fatalf("token 换取失败，got %d：%s", code, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		t.Fatalf("token 响应缺 access_token：%s", body)
	}

	// 5. userinfo 拉资料（sub/username/nickname/email/avatar）
	code, body = do(t, http.MethodGet, ts.URL+"/api/oauth/userinfo", tok.AccessToken, "")
	if code != http.StatusOK || strings.Contains(string(body), `"error"`) {
		t.Fatalf("userinfo 失败，got %d：%s", code, body)
	}
	var info map[string]any
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("userinfo 解析失败：%v", err)
	}
	for _, k := range []string{"sub", "username", "nickname", "email", "avatar"} {
		if _, ok := info[k]; !ok {
			t.Fatalf("userinfo 缺字段 %s：%s", k, body)
		}
	}

	// 6. 授权码一次性：重放同一 code 必须被拒（真实存储层的 TakeCode 原子核销）
	code, body = do(t, http.MethodPost, ts.URL+"/api/oauth/token", "",
		`{"client_id":"`+client.ClientID+`","client_secret":"`+client.ClientSecret+
			`","code":"`+authCode+`","redirect_uri":"https://app.example.com/auth/callback"}`)
	if code != http.StatusOK || !strings.Contains(string(body), `"error"`) {
		t.Fatalf("授权码重放应返回 error，got %d：%s", code, body)
	}
}
