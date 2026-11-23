package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/adapter"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// 编译期断言：假仓储满足 service 消费方定义的窄接口。
var _ instanceRepo = (*stubRepo)(nil)

// stubRepo 假仓储：内存实现 instanceRepo 窄接口，不依赖 MySQL。
type stubRepo struct {
	mu        sync.Mutex
	byID      map[uint]*model.ServiceInstance
	nextID    uint
	probes    []string // 记录每次 SaveProbeResult 的 status
	notFoundX bool     // 置 true 时 SaveProbeResult 返回错误（模拟 DB 故障）
}

func newStubRepo() *stubRepo {
	return &stubRepo{byID: map[uint]*model.ServiceInstance{}, nextID: 1}
}

func (r *stubRepo) List() ([]model.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ServiceInstance, 0, len(r.byID))
	for _, i := range r.byID {
		out = append(out, *i)
	}
	return out, nil
}

func (r *stubRepo) FindByID(id uint) (*model.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i, ok := r.byID[id]; ok {
		cp := *i
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *stubRepo) FindByName(name string) (*model.ServiceInstance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, i := range r.byID {
		if i.Name == name {
			cp := *i
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *stubRepo) Create(i *model.ServiceInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i.ID = r.nextID
	r.nextID++
	r.byID[i.ID] = i
	return nil
}

func (r *stubRepo) Save(i *model.ServiceInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[i.ID] = i
	return nil
}

func (r *stubRepo) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

func (r *stubRepo) SaveProbeResult(id uint, status string, _ *string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.notFoundX {
		return errors.New("db down")
	}
	r.probes = append(r.probes, status)
	return nil
}

var jwtSecretForTest = "test-jwt-secret-0123456789abcdef-0123456789"

func newTestService() (*InstanceService, *stubRepo) {
	return newTestServiceWith(nil)
}

// newTestServiceWith 可注入 adapter 注册表（更新编排测试用；nil=无任何 adapter）。
func newTestServiceWith(updates *adapter.Registry) (*InstanceService, *stubRepo) {
	repo := newStubRepo()
	return NewInstanceService(repo, crypto.NewBox("", jwtSecretForTest), updates), repo
}

func TestCreateValidation(t *testing.T) {
	svc, _ := newTestService()

	cases := []struct {
		name    string
		create  func() error
		wantErr error
	}{
		{"空名称", func() error {
			_, err := svc.Create("", "  ", "http://x.local", "none", "", "", "", true, "")
			return err
		}, apperr.ErrInvalidInput},
		{"超长名称", func() error {
			_, err := svc.Create("", strings.Repeat("名", 31), "http://x.local", "none", "", "", "", true, "")
			return err
		}, apperr.ErrInvalidInput},
		{"超长 kind", func() error {
			_, err := svc.Create(strings.Repeat("k", 31), "a", "http://x.local", "none", "", "", "", true, "")
			return err
		}, apperr.ErrInvalidInput},
		{"非 http 协议", func() error { _, err := svc.Create("", "a", "ftp://x.local", "none", "", "", "", true, ""); return err }, apperr.ErrBadBaseURL},
		{"缺主机名", func() error { _, err := svc.Create("", "a", "http://", "none", "", "", "", true, ""); return err }, apperr.ErrBadBaseURL},
		{"未知鉴权方式", func() error {
			_, err := svc.Create("", "a", "http://x.local", "basic", "", "", "", true, "")
			return err
		}, apperr.ErrAuthType},
		{"bearer 缺凭据", func() error {
			_, err := svc.Create("", "a", "http://x.local", "bearer", "", "", "", true, "")
			return err
		}, apperr.ErrInvalidInput},
	}
	for _, tc := range cases {
		if err := tc.create(); !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s: got %v, want %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestCreateDefaultsAndUnique(t *testing.T) {
	svc, _ := newTestService()

	inst, err := svc.Create("", " hub-self ", "http://127.0.0.1:8091/", "none", "", "healthz", "", false, " 说明 ")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}
	if inst.Kind != model.KindGeneric {
		t.Fatalf("空 kind 应归一为 generic: %q", inst.Kind)
	}
	if inst.Name != "hub-self" {
		t.Fatalf("名称应去空格: %q", inst.Name)
	}
	if inst.HealthPath != "/healthz" {
		t.Fatalf("健康路径应补前导斜杠: %q", inst.HealthPath)
	}
	if inst.LastStatus != "unknown" {
		t.Fatalf("初始状态应为 unknown: %q", inst.LastStatus)
	}
	if inst.Enabled {
		t.Fatal("enabled=false 应被保留")
	}

	// 空健康路径 → 默认 /api/health；kind 空值同上
	inst2, err := svc.Create("", "other", "http://127.0.0.1:8092", "none", "", "", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}
	if inst2.HealthPath != DefaultHealthPath {
		t.Fatalf("默认健康路径应为 %s: %q", DefaultHealthPath, inst2.HealthPath)
	}

	// 重名 → 409
	if _, err := svc.Create("", "hub-self", "http://127.0.0.1:8093", "none", "", "", "", true, ""); !errors.Is(err, apperr.ErrNameTaken) {
		t.Fatalf("重名应返回 ErrNameTaken: %v", err)
	}
}

func TestCreateBearerSealsToken(t *testing.T) {
	svc, _ := newTestService()
	box := crypto.NewBox("", jwtSecretForTest)

	inst, err := svc.Create("", "authed", "http://10.0.0.1", "bearer", "plain-token", "", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}
	if inst.TokenCiphertext == "" || inst.TokenCiphertext == "plain-token" {
		t.Fatal("凭据应加密落库且不等于明文")
	}
	if plain, err := box.Open(inst.TokenCiphertext); err != nil || plain != "plain-token" {
		t.Fatalf("密文应可解回明文: plain=%q err=%v", plain, err)
	}
}

func TestUpdateKeepsTokenWhenEmpty(t *testing.T) {
	svc, _ := newTestService()
	inst, err := svc.Create("", "a", "http://10.0.0.1", "bearer", "old-token", "", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}

	updated, err := svc.Update(inst.ID, "", "a2", "http://10.0.0.2", "bearer", "", "/api/health", "", false, "改备注")
	if err != nil {
		t.Fatalf("Update() 出错: %v", err)
	}
	if updated.TokenCiphertext == "" {
		t.Fatal("token 空值应保持原凭据")
	}
	if !strings.Contains(updated.Note, "改备注") {
		t.Fatalf("备注应更新: %q", updated.Note)
	}

	// 改成已有名称 → 409
	if _, err := svc.Create("", "b", "http://10.0.0.3", "none", "", "", "", true, ""); err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}
	if _, err := svc.Update(inst.ID, "", "b", "http://10.0.0.2", "none", "", "", "", true, ""); !errors.Is(err, apperr.ErrNameTaken) {
		t.Fatalf("改成重名应返回 ErrNameTaken: %v", err)
	}
}

func TestGetDeleteNotFound(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Get(999); !errors.Is(err, apperr.ErrInstanceNotFound) {
		t.Fatalf("Get 不存在应返回 ErrInstanceNotFound: %v", err)
	}
	if err := svc.Delete(999); err != nil {
		t.Fatalf("Delete 不存在应幂等成功: %v", err)
	}
}

func TestProbeAgainstFakeServer(t *testing.T) {
	var hitHealth, hitVersion bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			hitHealth = true
			w.WriteHeader(http.StatusOK)
		case "/api/version":
			hitVersion = true
			_, _ = w.Write([]byte(`{"version":"9.9.9","commit":"abc","buildTime":"now"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	svc, repo := newTestService()
	inst, err := svc.Create("", "fake", ts.URL, "none", "", "/healthz", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}

	got, err := svc.Probe(inst.ID)
	if err != nil {
		t.Fatalf("Probe() 出错: %v", err)
	}
	if !hitHealth || !hitVersion {
		t.Fatalf("健康与版本端点都应被访问: health=%v version=%v", hitHealth, hitVersion)
	}
	if got.LastStatus != "ok" {
		t.Fatalf("探测应 ok: %q", got.LastStatus)
	}
	if got.LastVersion == nil || *got.LastVersion != "9.9.9" {
		t.Fatalf("应记录版本 9.9.9: %v", got.LastVersion)
	}
	if got.LastCheckAt == nil {
		t.Fatal("LastCheckAt 应被写入")
	}
	if len(repo.probes) != 1 || repo.probes[0] != "ok" {
		t.Fatalf("探测结果应写回仓储: %v", repo.probes)
	}
}

func TestProbeAllBoundedConcurrent(t *testing.T) {
	// 一个 ok 一个 fail：批量探测应全部写回且逐实例状态正确
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	tsBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsBad.Close()

	svc, repo := newTestService()
	if _, err := svc.Create("", "ok1", ts.URL, "none", "", "/api/health", "", true, ""); err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}
	if _, err := svc.Create("", "bad1", tsBad.URL, "none", "", "/api/health", "", true, ""); err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}

	results, err := svc.ProbeAll()
	if err != nil {
		t.Fatalf("ProbeAll() 出错: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应有两条结果: %d", len(results))
	}
	byName := map[string]ProbeAllResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if r, ok := byName["ok1"]; !ok || r.LastStatus != "ok" || r.LastVersion == nil || *r.LastVersion != "1.0.0" {
		t.Fatalf("ok1 结果不符: %+v", r)
	}
	if r, ok := byName["bad1"]; !ok || r.LastStatus != "fail" {
		t.Fatalf("bad1 结果不符: %+v", r)
	}
	if len(repo.probes) != 2 {
		t.Fatalf("两次探测都应写回: %v", repo.probes)
	}
}

func TestProbeFailuresAreData(t *testing.T) {
	// 500 → fail（版本端点缺失也应 ok，版本可选）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc, _ := newTestService()
	inst, _ := svc.Create("", "bad", ts.URL, "none", "", "/api/health", "", true, "")
	got, err := svc.Probe(inst.ID)
	if err != nil {
		t.Fatalf("探测失败应作为数据返回，而不是错误: %v", err)
	}
	if got.LastStatus != "fail" {
		t.Fatalf("应记录 fail: %q", got.LastStatus)
	}

	// 连不上 → fail
	inst2, _ := svc.Create("", "dead", "http://127.0.0.1:1", "none", "", "/healthz", "", true, "")
	got2, err := svc.Probe(inst2.ID)
	if err != nil {
		t.Fatalf("探测失败应作为数据返回，而不是错误: %v", err)
	}
	if got2.LastStatus != "fail" {
		t.Fatalf("连不上应记录 fail: %q", got2.LastStatus)
	}

	// 版本端点非 JSON 时健康 ok → ok 但版本为空
	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsOK.Close()
	svc2, _ := newTestService()
	inst3, _ := svc2.Create("", "ok-only", tsOK.URL, "none", "", "/api/health", "", true, "")
	got3, err := svc2.Probe(inst3.ID)
	if err != nil || got3.LastStatus != "ok" {
		t.Fatalf("健康 ok 应整体 ok: status=%q err=%v", got3.LastStatus, err)
	}
	if got3.LastVersion != nil {
		t.Fatalf("版本解析失败应为 nil: %v", got3.LastVersion)
	}
}

// ── 更新编排（adapter 窄接口注入）──

// stubUpdater 假 adapter：记录收到的 base/token，返回预设结果。
type stubUpdater struct {
	kind              string
	gotBase, gotToken string
	check             *adapter.UpdateCheck
	err               error
	starts            int
}

func (u *stubUpdater) Kind() string { return u.kind }

func (u *stubUpdater) Check(baseURL, token string) (*adapter.UpdateCheck, error) {
	u.gotBase, u.gotToken = baseURL, token
	return u.check, u.err
}

func (u *stubUpdater) Status(baseURL, token string) (*adapter.UpdateStatus, error) {
	u.gotBase, u.gotToken = baseURL, token
	return &adapter.UpdateStatus{Available: true, Busy: false, Phase: "idle"}, u.err
}

func (u *stubUpdater) Start(baseURL, token string) error {
	u.starts++
	u.gotBase, u.gotToken = baseURL, token
	return u.err
}

func TestUpdateOrchestration(t *testing.T) {
	reg := adapter.NewRegistry()
	stub := &stubUpdater{kind: "modernchat-new", check: &adapter.UpdateCheck{
		CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0", UpdateAvailable: true,
	}}
	reg.RegisterUpdater(stub)
	svc, _ := newTestServiceWith(reg)

	// bearer 实例：adapter 应收到解密后的明文凭据
	inst, err := svc.Create("modernchat-new", "mcn", "http://10.0.0.9", "bearer", "target-admin-token", "", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}

	check, err := svc.UpdateCheck(inst.ID)
	if err != nil {
		t.Fatalf("UpdateCheck() 出错: %v", err)
	}
	if !check.UpdateAvailable || check.LatestVersion != "v1.1.0" {
		t.Fatalf("检查结果应透传: %+v", check)
	}
	if stub.gotBase != inst.BaseURL || stub.gotToken != "target-admin-token" {
		t.Fatalf("adapter 应收到 base 与解密后的凭据: base=%q token=%q", stub.gotBase, stub.gotToken)
	}

	status, err := svc.UpdateStatus(inst.ID)
	if err != nil {
		t.Fatalf("UpdateStatus() 出错: %v", err)
	}
	if !status.Available || status.Phase != "idle" {
		t.Fatalf("状态应透传: %+v", status)
	}

	if err := svc.UpdateStart(inst.ID); err != nil {
		t.Fatalf("UpdateStart() 出错: %v", err)
	}
	if stub.starts != 1 {
		t.Fatalf("Start 应被调用一次: %d", stub.starts)
	}

	// 实例不存在
	if _, err := svc.UpdateCheck(999); !errors.Is(err, apperr.ErrInstanceNotFound) {
		t.Fatalf("不存在实例应返回 ErrInstanceNotFound: %v", err)
	}
}

func TestUpdateUnknownKind(t *testing.T) {
	reg := adapter.NewRegistry()
	reg.RegisterUpdater(&stubUpdater{kind: "modernchat-new"})
	svc, _ := newTestServiceWith(reg)

	// generic：无 adapter，更新操作一律不支持
	g, _ := svc.Create("", "generic-inst", "http://10.0.0.1", "none", "", "", "", true, "")
	if _, err := svc.UpdateCheck(g.ID); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("generic 应返回 ErrKindUnknown: %v", err)
	}
	if err := svc.UpdateStart(g.ID); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("generic 启动更新应返回 ErrKindUnknown: %v", err)
	}

	// 未注册的其他 kind 同样不支持
	o, _ := svc.Create("some-other", "other-inst", "http://10.0.0.2", "none", "", "", "", true, "")
	if _, err := svc.UpdateStatus(o.ID); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("未注册 kind 应返回 ErrKindUnknown: %v", err)
	}

	// 无注册表（nil）时同理
	svc2, _ := newTestServiceWith(nil)
	n, _ := svc2.Create("modernchat-new", "no-registry", "http://10.0.0.3", "none", "", "", "", true, "")
	if err := svc2.UpdateStart(n.ID); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("无注册表应返回 ErrKindUnknown: %v", err)
	}
}

func TestUpdateAdapterErrorPassthrough(t *testing.T) {
	// adapter 返回的语义哨兵应原样上抛（handler 负责状态码映射）
	reg := adapter.NewRegistry()
	stub := &stubUpdater{kind: "modernchat-new", err: apperr.ErrTargetAuth}
	reg.RegisterUpdater(stub)
	svc, _ := newTestServiceWith(reg)

	inst, _ := svc.Create("modernchat-new", "mcn", "http://10.0.0.9", "none", "", "", "", true, "")
	if _, err := svc.UpdateCheck(inst.ID); !errors.Is(err, apperr.ErrTargetAuth) {
		t.Fatalf("adapter 错误应透传: %v", err)
	}
}

// ── 管理操作编排（统一管理后台）──

// stubAdminer 假管理 adapter：记录收到的 base/token 与动作参数。
type stubAdminer struct {
	kind              string
	gotBase, gotToken string
	stats             *adapter.AdminStats
	users             *adapter.AdminUserPage
	bannedID, kicked  uint
	resetID           uint
	newPassword       string
	err               error
}

func (a *stubAdminer) Kind() string { return a.kind }

func (a *stubAdminer) Stats(baseURL, token string) (*adapter.AdminStats, error) {
	a.gotBase, a.gotToken = baseURL, token
	return a.stats, a.err
}

func (a *stubAdminer) Users(baseURL, token string, page, pageSize int, kw, role, status string) (*adapter.AdminUserPage, error) {
	a.gotBase, a.gotToken = baseURL, token
	return a.users, a.err
}

func (a *stubAdminer) Ban(baseURL, token string, userID uint, _ int, _ string) error {
	a.gotBase, a.gotToken, a.bannedID = baseURL, token, userID
	return a.err
}

func (a *stubAdminer) Unban(baseURL, token string, userID uint) error { return a.err }

func (a *stubAdminer) Kick(baseURL, token string, userID uint) (bool, error) {
	a.gotBase, a.gotToken, a.kicked = baseURL, token, userID
	return true, a.err
}

func (a *stubAdminer) ResetPassword(baseURL, token string, userID uint) (string, error) {
	a.resetID = userID
	return a.newPassword, a.err
}

// 编译期断言：假管理 adapter 满足窄接口。
var _ adapter.Adminer = (*stubAdminer)(nil)

func TestAdminOrchestration(t *testing.T) {
	reg := adapter.NewRegistry()
	adminer := &stubAdminer{
		kind:  "modernchat-new",
		stats: &adapter.AdminStats{UsersTotal: 42, Online: 3, MessagesTotal: 1024},
		users: &adapter.AdminUserPage{Total: 1, Page: 1, PageSize: 20, Users: []adapter.AdminUser{
			{ID: 7, Username: "alice", Nickname: "Alice", Role: "user"},
		}},
		newPassword: "Rand0mPwd12",
	}
	reg.RegisterAdmin(adminer)
	svc, _ := newTestServiceWith(reg)

	// bearer 实例：adapter 应收到解密后的明文凭据
	inst, err := svc.Create("modernchat-new", "admin-mcn", "http://10.0.0.8", "bearer", "target-admin-token", "", "", true, "")
	if err != nil {
		t.Fatalf("Create() 出错: %v", err)
	}

	stats, err := svc.AdminStats(inst.ID)
	if err != nil {
		t.Fatalf("AdminStats() 出错: %v", err)
	}
	if stats.UsersTotal != 42 || stats.Online != 3 {
		t.Fatalf("统计透传不符: %+v", stats)
	}
	if adminer.gotBase != inst.BaseURL || adminer.gotToken != "target-admin-token" {
		t.Fatalf("adapter 应收到 base 与解密后的凭据: base=%q token=%q", adminer.gotBase, adminer.gotToken)
	}

	page, err := svc.AdminUsers(inst.ID, 1, 20, "ali", "", "")
	if err != nil {
		t.Fatalf("AdminUsers() 出错: %v", err)
	}
	if page.Total != 1 || len(page.Users) != 1 || page.Users[0].Username != "alice" {
		t.Fatalf("用户分页透传不符: %+v", page)
	}

	if err := svc.AdminBan(inst.ID, 7, 3, "违规"); err != nil {
		t.Fatalf("AdminBan() 出错: %v", err)
	}
	if adminer.bannedID != 7 {
		t.Fatalf("封禁应作用于目标用户 7: %d", adminer.bannedID)
	}

	wasOnline, err := svc.AdminKick(inst.ID, 7)
	if err != nil || !wasOnline {
		t.Fatalf("AdminKick() = %v, %v", wasOnline, err)
	}

	pwd, err := svc.AdminResetPassword(inst.ID, 7)
	if err != nil || pwd != "Rand0mPwd12" || adminer.resetID != 7 {
		t.Fatalf("AdminResetPassword() = %q, %v", pwd, err)
	}

	// generic 实例：无 adminer
	g, _ := svc.Create("", "generic-admin", "http://10.0.0.7", "none", "", "", "", true, "")
	if _, err := svc.AdminStats(g.ID); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("generic 应返回 ErrKindUnknown: %v", err)
	}
	if err := svc.AdminBan(g.ID, 1, 3, ""); !errors.Is(err, apperr.ErrKindUnknown) {
		t.Fatalf("generic 封禁应返回 ErrKindUnknown: %v", err)
	}
}
