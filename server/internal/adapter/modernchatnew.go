package adapter

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// KindModernchatNew 主仓库（modern-chat-new）的服务类型标识。
const KindModernchatNew = "modernchat-new"

// 主仓库在线更新接口的响应形状（internal/service/updater.go 的 StatusView / CheckResult）。
// 项目组 JWT 口径一致：凭 Bearer 管理员 token 调用 /api/admin/update/*。
type targetStatusView struct {
	Available bool     `json:"available"`
	Busy      bool     `json:"busy"`
	Phase     string   `json:"phase"`
	Lines     []string `json:"lines"`
	Error     string   `json:"error,omitempty"`
	Version   string   `json:"version"`
	Commit    string   `json:"commit"`
	Mode      string   `json:"mode"` // release | source
}

type targetCheckResult struct {
	UpToDate  bool   `json:"upToDate"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	RemoteTag string `json:"remoteTag"`
}

// ModernchatNew 主仓库更新 adapter：代理其站长后台的检查 / 状态 / 启动更新。
// 目标 WEB_UPDATE=false 或部署方式不支持时，Check/Start 返回 400（原因透传）、
// Status 返回 {available:false}——adapter 归一为语义哨兵，不做猜测。
type ModernchatNew struct {
	http *http.Client
}

// NewModernchatNew 构造（15s 超时：目标 Check 会在服务端拉 Release 源）。
func NewModernchatNew() *ModernchatNew {
	return &ModernchatNew{http: &http.Client{Timeout: 15 * time.Second}}
}

// Kind 实现 Updater。
func (a *ModernchatNew) Kind() string { return KindModernchatNew }

// Check 检查目标是否有新版本（目标本地版本 vs Release 源最新 tag）。
func (a *ModernchatNew) Check(baseURL, token string) (*UpdateCheck, error) {
	var r targetCheckResult
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/update/check"), token, &r); err != nil {
		return nil, err
	}
	return &UpdateCheck{
		CurrentVersion:  r.Local,
		LatestVersion:   r.Remote,
		UpdateAvailable: !r.UpToDate,
	}, nil
}

// Status 查询目标更新任务状态（Web 端轮询用）。
func (a *ModernchatNew) Status(baseURL, token string) (*UpdateStatus, error) {
	var r targetStatusView
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/update/status"), token, &r); err != nil {
		return nil, err
	}
	return &UpdateStatus{
		Available:      r.Available,
		Busy:           r.Busy,
		Phase:          r.Phase,
		Lines:          r.Lines,
		Error:          r.Error,
		CurrentVersion: r.Version,
		Mode:           r.Mode,
	}, nil
}

// Start 触发目标在线更新（异步执行，进度靠 Status 轮询）。
func (a *ModernchatNew) Start(baseURL, token string) error {
	var r struct {
		Started bool `json:"started"`
	}
	if err := requestJSON(a.http, http.MethodPost, joinURL(baseURL, "/api/admin/update/start"), token, &r); err != nil {
		return err
	}
	if !r.Started {
		return errors.New("目标服务未确认启动更新")
	}
	return nil
}

// ── 管理操作（Adminer）：代理主仓库站长后台 /api/admin/* ──

// targetStatsView 主仓库 GET /api/admin/stats 的响应形状（需求规格 §3.2）。
type targetStatsView struct {
	Users struct {
		Total  int64 `json:"total"`
		Today  int64 `json:"today"`
		Banned int64 `json:"banned"`
		Admins int64 `json:"admins"`
	} `json:"users"`
	Conversations struct {
		Single int64 `json:"single"`
		Group  int64 `json:"group"`
	} `json:"conversations"`
	Messages struct {
		Total int64 `json:"total"`
		Today int64 `json:"today"`
	} `json:"messages"`
	Plugins struct {
		Installed int64 `json:"installed"`
		Enabled   int64 `json:"enabled"`
		Disabled  int64 `json:"disabled"`
	} `json:"plugins"`
	Online        int64  `json:"online"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	GoVersion     string `json:"goVersion"`
}

// Stats 实现 Adminer：目标概览统计。
func (a *ModernchatNew) Stats(baseURL, token string) (*AdminStats, error) {
	var r targetStatsView
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/stats"), token, &r); err != nil {
		return nil, err
	}
	return &AdminStats{
		UsersTotal:       r.Users.Total,
		UsersToday:       r.Users.Today,
		UsersBanned:      r.Users.Banned,
		UsersAdmins:      r.Users.Admins,
		Conversations:    r.Conversations.Single + r.Conversations.Group,
		MessagesTotal:    r.Messages.Total,
		MessagesToday:    r.Messages.Today,
		PluginsInstalled: r.Plugins.Installed,
		Online:           r.Online,
		UptimeSeconds:    r.UptimeSeconds,
		GoVersion:        r.GoVersion,
	}, nil
}

// targetAdminUser 主仓库用户行（AdminUserRow = User + createdAt；指针字段 nil 时不出现）。
type targetAdminUser struct {
	ID          uint    `json:"id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	AvatarColor string  `json:"avatarColor"`
	Email       *string `json:"email"`
	Role        string  `json:"role"`
	BannedUntil *string `json:"bannedUntil"`
	BanReason   *string `json:"banReason"`
	CreatedAt   string  `json:"createdAt"`
}

func (u targetAdminUser) normalize() AdminUser {
	out := AdminUser{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		AvatarColor: u.AvatarColor,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
	}
	if u.Email != nil {
		out.Email = *u.Email
	}
	if u.BannedUntil != nil {
		out.BannedUntil = *u.BannedUntil
	}
	if u.BanReason != nil {
		out.BanReason = *u.BanReason
	}
	return out
}

// targetAdminUserPage 主仓库 GET /api/admin/users 的响应形状。
type targetAdminUserPage struct {
	Users    []targetAdminUser `json:"users"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// Users 实现 Adminer：目标用户分页列表（kw / role / status 筛选透传）。
func (a *ModernchatNew) Users(baseURL, token string, page, pageSize int, kw, role, status string) (*AdminUserPage, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	if kw != "" {
		q.Set("kw", kw)
	}
	if role != "" {
		q.Set("role", role)
	}
	if status != "" {
		q.Set("status", status)
	}
	var r targetAdminUserPage
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/users?"+q.Encode()), token, &r); err != nil {
		return nil, err
	}
	out := &AdminUserPage{Total: r.Total, Page: r.Page, PageSize: r.PageSize, Users: make([]AdminUser, 0, len(r.Users))}
	for _, u := range r.Users {
		out.Users = append(out.Users, u.normalize())
	}
	return out, nil
}

// Ban 实现 Adminer：封禁目标用户（days=0 永久）。
func (a *ModernchatNew) Ban(baseURL, token string, userID uint, days int, reason string) error {
	body := map[string]any{"days": days}
	if reason != "" {
		body["reason"] = reason
	}
	var r struct {
		OK bool `json:"ok"`
	}
	return requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, fmt.Sprintf("/api/admin/users/%d/ban", userID)), token, body, &r)
}

// Unban 实现 Adminer：解封。
func (a *ModernchatNew) Unban(baseURL, token string, userID uint) error {
	var r struct {
		OK bool `json:"ok"`
	}
	return requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, fmt.Sprintf("/api/admin/users/%d/unban", userID)), token, map[string]any{}, &r)
}

// Kick 实现 Adminer：强制下线（不封禁），返回目标端 wasOnline。
func (a *ModernchatNew) Kick(baseURL, token string, userID uint) (bool, error) {
	var r struct {
		OK        bool `json:"ok"`
		WasOnline bool `json:"wasOnline"`
	}
	if err := requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, fmt.Sprintf("/api/admin/users/%d/kick", userID)), token, map[string]any{}, &r); err != nil {
		return false, err
	}
	return r.WasOnline, nil
}

// ResetPassword 实现 Adminer：目标端随机重置密码（返回一次性明文，Hub 不落库）。
func (a *ModernchatNew) ResetPassword(baseURL, token string, userID uint) (string, error) {
	var r struct {
		NewPassword string `json:"newPassword"`
	}
	if err := requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, fmt.Sprintf("/api/admin/users/%d/reset-password", userID)), token, map[string]any{}, &r); err != nil {
		return "", err
	}
	if r.NewPassword == "" {
		return "", errors.New("目标服务未返回新密码")
	}
	return r.NewPassword, nil
}

// targetAdminPluginPage 主仓库 GET /api/admin/plugins 的响应形状（规格 §3.2）。
type targetAdminPluginPage struct {
	Plugins  []AdminPlugin `json:"plugins"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// Plugins 实现 Adminer：目标插件聚合列表。
func (a *ModernchatNew) Plugins(baseURL, token string, page, pageSize int) (*AdminPluginPage, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	var r targetAdminPluginPage
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/plugins?"+q.Encode()), token, &r); err != nil {
		return nil, err
	}
	if r.Plugins == nil {
		r.Plugins = []AdminPlugin{}
	}
	return &AdminPluginPage{Plugins: r.Plugins, Total: r.Total, Page: r.Page, PageSize: r.PageSize}, nil
}

// SetPluginGlobalDisabled 实现 Adminer：站长全局下架 / 恢复某插件。
func (a *ModernchatNew) SetPluginGlobalDisabled(baseURL, token, name string, disabled bool) (int64, error) {
	var r struct {
		OK       bool  `json:"ok"`
		Affected int64 `json:"affected"`
	}
	err := requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, "/api/admin/plugins/global-disable"), token,
		map[string]any{"name": name, "disabled": disabled}, &r)
	if err != nil {
		return 0, err
	}
	if !r.OK {
		return 0, errors.New("目标服务未确认插件下架操作")
	}
	return r.Affected, nil
}

// targetAdminAuditPage 主仓库 GET /api/admin/audits 的响应形状。
type targetAdminAuditPage struct {
	Audits   []AdminAudit `json:"audits"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

// Audits 实现 Adminer：目标审计日志分页。
func (a *ModernchatNew) Audits(baseURL, token string, page, pageSize int) (*AdminAuditPage, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	var r targetAdminAuditPage
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/admin/audits?"+q.Encode()), token, &r); err != nil {
		return nil, err
	}
	if r.Audits == nil {
		r.Audits = []AdminAudit{}
	}
	return &AdminAuditPage{Audits: r.Audits, Total: r.Total, Page: r.Page, PageSize: r.PageSize}, nil
}

// Announce 实现 Adminer：向目标服务发送全站公告。
func (a *ModernchatNew) Announce(baseURL, token, content string) (*AnnounceResult, error) {
	var r AnnounceResult
	if err := requestJSONBody(a.http, http.MethodPost, joinURL(baseURL, "/api/admin/announce"), token,
		map[string]any{"content": content}, &r); err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, errors.New("目标服务未确认公告发送")
	}
	return &r, nil
}
