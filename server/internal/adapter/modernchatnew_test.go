package adapter

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
)

// fakeTarget 起一个模仿主仓库 /api/admin/update/* 行为的假目标。
func fakeTarget(handler http.HandlerFunc) *httptest.Server {
	ts := httptest.NewServer(handler)
	return ts
}

func TestCheckAvailable(t *testing.T) {
	var gotAuth string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admin/update/check" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"upToDate":false,"local":"v1.0.0","remote":"v1.1.0","remoteTag":"v1.1.0","remoteMsg":"v1.1.0"}`))
	}))
	defer ts.Close()

	a := NewModernchatNew()
	res, err := a.Check(ts.URL, "admin-token")
	if err != nil {
		t.Fatalf("Check() 出错: %v", err)
	}
	if gotAuth != "Bearer admin-token" {
		t.Fatalf("应携带 Bearer 凭据: %q", gotAuth)
	}
	if res.CurrentVersion != "v1.0.0" || res.LatestVersion != "v1.1.0" || !res.UpdateAvailable {
		t.Fatalf("归一结果不符: %+v", res)
	}

	// upToDate=true → UpdateAvailable=false
	ts2 := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"upToDate":true,"local":"v1.1.0","remote":"v1.1.0"}`))
	}))
	defer ts2.Close()
	res2, err := a.Check(ts2.URL, "")
	if err != nil {
		t.Fatalf("Check() 出错: %v", err)
	}
	if res2.UpdateAvailable {
		t.Fatal("upToDate 时不应有可用更新")
	}
}

func TestStatusMapping(t *testing.T) {
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admin/update/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"available":true,"busy":true,"phase":"downloading","lines":["下载中 40%"],"version":"v1.0.0","mode":"release"}`))
	}))
	defer ts.Close()

	a := NewModernchatNew()
	s, err := a.Status(ts.URL, "t")
	if err != nil {
		t.Fatalf("Status() 出错: %v", err)
	}
	if !s.Available || !s.Busy || s.Phase != "downloading" || s.CurrentVersion != "v1.0.0" || s.Mode != "release" {
		t.Fatalf("状态归一不符: %+v", s)
	}
	if len(s.Lines) != 1 || s.Lines[0] != "下载中 40%" {
		t.Fatalf("日志行应透传: %v", s.Lines)
	}
}

func TestStartOK(t *testing.T) {
	var method string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_, _ = w.Write([]byte(`{"started":true}`))
	}))
	defer ts.Close()

	a := NewModernchatNew()
	if err := a.Start(ts.URL, "t"); err != nil {
		t.Fatalf("Start() 出错: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("Start 应为 POST: %s", method)
	}
}

func TestTargetErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
		wantMsg string // 错误消息应包含的目标原因片段（空串=不校验）
	}{
		{"401 凭据失效", http.StatusUnauthorized, `{"error":"登录已失效"}`, apperr.ErrTargetAuth, "登录已失效"},
		{"403 非管理员", http.StatusForbidden, `{"error":"无权执行此操作"}`, apperr.ErrTargetAuth, "无权执行此操作"},
		{"404 接口不存在", http.StatusNotFound, `404 page not found`, apperr.ErrUpdateUnsupported, ""},
		{"409 任务进行中", http.StatusConflict, `{"error":"有更新任务正在进行中，请稍候"}`, apperr.ErrUpdateBusy, "有更新任务正在进行中"},
		{"400 部署不支持", http.StatusBadRequest, `{"error":"当前部署方式不支持在线更新（需要 scripts/install.sh 的 systemd 部署）"}`, apperr.ErrUpdateUnsupported, "systemd"},
		{"500 内部错误", http.StatusInternalServerError, `boom`, apperr.ErrTargetUnreachable, "boom"},
	}
	for _, tc := range cases {
		ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		a := NewModernchatNew()
		_, err := a.Check(ts.URL, "t")
		ts.Close()
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s: got %v, want %v", tc.name, err, tc.wantErr)
		}
		if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
			t.Fatalf("%s: 错误消息应包含目标原因 %q: %v", tc.name, tc.wantMsg, err)
		}
	}
}

func TestTargetUnreachable(t *testing.T) {
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close() // 先关闭制造不可达

	a := NewModernchatNew()
	if _, err := a.Check(url, ""); !errors.Is(err, apperr.ErrTargetUnreachable) {
		t.Fatalf("不可达应返回 ErrTargetUnreachable: %v", err)
	}
}

// ── 管理操作（Adminer）方法测试 ──

func TestAdminStatsMapping(t *testing.T) {
	var gotAuth string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admin/stats" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"users":{"total":42,"today":2,"banned":1,"admins":1},"conversations":{"single":10,"group":5},"messages":{"total":1000,"today":30},"plugins":{"installed":3,"enabled":2,"disabled":1},"online":7,"uptimeSeconds":3600,"goVersion":"go1.26.3"}`))
	}))
	defer ts.Close()

	a := NewModernchatNew()
	s, err := a.Stats(ts.URL, "admin-token")
	if err != nil {
		t.Fatalf("Stats() 出错: %v", err)
	}
	if gotAuth != "Bearer admin-token" {
		t.Fatalf("应携带 Bearer 凭据: %q", gotAuth)
	}
	if s.UsersTotal != 42 || s.UsersBanned != 1 || s.Conversations != 15 || s.MessagesTotal != 1000 || s.PluginsInstalled != 3 || s.Online != 7 || s.UptimeSeconds != 3600 || s.GoVersion != "go1.26.3" {
		t.Fatalf("统计归一不符: %+v", s)
	}
}

func TestAdminUsersMapping(t *testing.T) {
	var gotQuery string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admin/users" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"users":[{"id":7,"username":"alice","nickname":"Alice","avatarColor":"#0052d9","email":"a@x.com","role":"user","createdAt":"2026-11-21T00:00:00Z"},{"id":8,"username":"bob","nickname":"Bob","role":"admin","bannedUntil":"2026-12-01T00:00:00Z","banReason":"违规"}],"total":2,"page":1,"pageSize":20}`))
	}))
	defer ts.Close()

	a := NewModernchatNew()
	p, err := a.Users(ts.URL, "t", 2, 50, "ali", "user", "banned")
	if err != nil {
		t.Fatalf("Users() 出错: %v", err)
	}
	// 查询串应包含分页与筛选
	for _, want := range []string{"page=2", "pageSize=50", "kw=ali", "role=user", "status=banned"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("查询串缺 %s: %q", want, gotQuery)
		}
	}
	if p.Total != 2 || p.Page != 1 || p.PageSize != 20 || len(p.Users) != 2 {
		t.Fatalf("分页形状不符: %+v", p)
	}
	alice, bob := p.Users[0], p.Users[1]
	if alice.Email != "a@x.com" || alice.BannedUntil != "" {
		t.Fatalf("alice 归一不符: %+v", alice)
	}
	if bob.Email != "" || bob.BannedUntil != "2026-12-01T00:00:00Z" || bob.BanReason != "违规" {
		t.Fatalf("bob 归一不符: %+v", bob)
	}
}

func TestAdminActions(t *testing.T) {
	var bodies []string
	var paths []string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		buf, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(buf))
		switch {
		case strings.HasSuffix(r.URL.Path, "/ban"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.HasSuffix(r.URL.Path, "/kick"):
			_, _ = w.Write([]byte(`{"ok":true,"wasOnline":true}`))
		case strings.HasSuffix(r.URL.Path, "/reset-password"):
			_, _ = w.Write([]byte(`{"newPassword":"Rand0mPwd12"}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer ts.Close()

	a := NewModernchatNew()
	if err := a.Ban(ts.URL, "t", 7, 3, "违规"); err != nil {
		t.Fatalf("Ban() 出错: %v", err)
	}
	if err := a.Unban(ts.URL, "t", 7); err != nil {
		t.Fatalf("Unban() 出错: %v", err)
	}
	wasOnline, err := a.Kick(ts.URL, "t", 8)
	if err != nil || !wasOnline {
		t.Fatalf("Kick() = %v, %v", wasOnline, err)
	}
	pwd, err := a.ResetPassword(ts.URL, "t", 8)
	if err != nil || pwd != "Rand0mPwd12" {
		t.Fatalf("ResetPassword() = %q, %v", pwd, err)
	}
	// 路径与方法核对
	want := []string{
		"POST /api/admin/users/7/ban",
		"POST /api/admin/users/7/unban",
		"POST /api/admin/users/8/kick",
		"POST /api/admin/users/8/reset-password",
	}
	for i, w := range want {
		if paths[i] != w {
			t.Fatalf("第 %d 个请求应为 %s: %s", i, w, paths[i])
		}
	}
	if !strings.Contains(bodies[0], `"days":3`) || !strings.Contains(bodies[0], `"reason":"违规"`) {
		t.Fatalf("封禁请求体不符: %s", bodies[0])
	}
}
