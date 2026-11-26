package adapter

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
)

// TestSSOCheck sso 的 Check 是 POST /api/v1/admin/update/checks，扩展字段（remoteMsg 等）应被忽略。
func TestSSOCheck(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"upToDate":false,"local":"v0.1.0","remote":"v0.2.0","remoteTag":"v0.2.0","remoteMsg":"v0.2.0","remoteNotes":"新版本说明"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	res, err := a.Check(ts.URL, "admin-token")
	if err != nil {
		t.Fatalf("Check() 出错: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/admin/update/checks" {
		t.Fatalf("sso Check 应为 POST /api/v1/admin/update/checks: %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer admin-token" {
		t.Fatalf("应携带 Bearer 凭据: %q", gotAuth)
	}
	if res.CurrentVersion != "v0.1.0" || res.LatestVersion != "v0.2.0" || !res.UpdateAvailable {
		t.Fatalf("归一结果不符: %+v", res)
	}

	// upToDate=true → UpdateAvailable=false
	ts2 := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"upToDate":true,"local":"v0.2.0","remote":"v0.2.0"}`))
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

// TestSSOStatus sso StatusView 无 mode 字段，归一后 Mode 应留空。
func TestSSOStatus(t *testing.T) {
	var gotPath string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"available":true,"busy":true,"phase":"downloading","lines":["下载中 40%"],"error":"","version":"v0.1.0","commit":"abc1234","repo":"https://github.com/modern-day-team/Modernchat-sso"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	s, err := a.Status(ts.URL, "t")
	if err != nil {
		t.Fatalf("Status() 出错: %v", err)
	}
	if gotPath != "/api/v1/admin/update" {
		t.Fatalf("sso Status 应为 GET /api/v1/admin/update: %s", gotPath)
	}
	if !s.Available || !s.Busy || s.Phase != "downloading" || s.CurrentVersion != "v0.1.0" {
		t.Fatalf("状态归一不符: %+v", s)
	}
	if s.Mode != "" {
		t.Fatalf("sso 无 mode 字段，归一后应留空: %q", s.Mode)
	}
	if len(s.Lines) != 1 || s.Lines[0] != "下载中 40%" {
		t.Fatalf("日志行应透传: %v", s.Lines)
	}
}

// TestSSOStart sso Start 返回 202 + 状态快照（无 started 字段），2xx 即视为已受理。
func TestSSOStart(t *testing.T) {
	var gotMethod, gotPath string
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"available":true,"busy":true,"phase":"started","version":"v0.1.0"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	if err := a.Start(ts.URL, "t"); err != nil {
		t.Fatalf("Start() 出错: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/admin/update" {
		t.Fatalf("sso Start 应为 POST /api/v1/admin/update: %s %s", gotMethod, gotPath)
	}
}

// TestSSOStartBusy 目标 409 → ErrUpdateBusy（decodeResponse 归一）。
func TestSSOStartBusy(t *testing.T) {
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"已有更新任务在进行中"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	err := a.Start(ts.URL, "t")
	if !errors.Is(err, apperr.ErrUpdateBusy) {
		t.Fatalf("409 应归一为 ErrUpdateBusy: %v", err)
	}
}

// TestSSOStartUnavailable 目标 400（部署方式不支持）→ ErrUpdateUnsupported。
func TestSSOStartUnavailable(t *testing.T) {
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"当前部署方式不支持在线更新"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	err := a.Start(ts.URL, "t")
	if !errors.Is(err, apperr.ErrUpdateUnsupported) {
		t.Fatalf("400 应归一为 ErrUpdateUnsupported: %v", err)
	}
	if got := fmt.Sprintf("%v", err); got == "" {
		t.Fatal("错误应携带可读信息")
	}
}

// TestSSOUnauth 401 → ErrTargetAuth（Hub 侧凭据对目标失效）。
func TestSSOUnauth(t *testing.T) {
	ts := fakeTarget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"未授权"}`))
	}))
	defer ts.Close()

	a := NewModernchatSSO()
	if _, err := a.Status(ts.URL, "expired"); !errors.Is(err, apperr.ErrTargetAuth) {
		t.Fatalf("401 应归一为 ErrTargetAuth: %v", err)
	}
}
