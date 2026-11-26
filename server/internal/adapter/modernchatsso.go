package adapter

import (
	"net/http"
	"time"
)

// KindModernchatSSO SSO 服务（Modernchat-sso）的服务类型标识。
const KindModernchatSSO = "modernchat-sso"

// ModernchatSSO SSO 服务更新 adapter：代理其站长后台的检查 / 状态 / 启动更新。
// sso 的更新接口与主仓库同形但前缀不同（router.go：/api/v1/admin/update[/checks]），
// 且响应形状有两处差异——Check 用 POST、Start 返回 202 + 状态快照（无 started 字段），
// 故独立 adapter 而非复用主仓库路径。凭 Bearer 管理员 token 调用。
type ModernchatSSO struct {
	http *http.Client
}

// NewModernchatSSO 构造（15s 超时：目标 Check 会在服务端拉 Release 源）。
func NewModernchatSSO() *ModernchatSSO {
	return &ModernchatSSO{http: &http.Client{Timeout: 15 * time.Second}}
}

// Kind 实现 Updater。
func (a *ModernchatSSO) Kind() string { return KindModernchatSSO }

// Check 检查目标是否有新版本（POST /api/v1/admin/update/checks）。
// sso CheckResult 与主仓库同形（upToDate/local/remote/remoteTag，另有 remoteMsg 等扩展字段忽略）。
func (a *ModernchatSSO) Check(baseURL, token string) (*UpdateCheck, error) {
	var r targetCheckResult
	if err := requestJSON(a.http, http.MethodPost, joinURL(baseURL, "/api/v1/admin/update/checks"), token, &r); err != nil {
		return nil, err
	}
	return &UpdateCheck{
		CurrentVersion:  r.Local,
		LatestVersion:   r.Remote,
		UpdateAvailable: !r.UpToDate,
	}, nil
}

// Status 查询目标更新任务状态（GET /api/v1/admin/update）。
// sso StatusView 与主仓库同形但无 mode 字段（该字段留空，Web 端按未知模式展示原文）。
func (a *ModernchatSSO) Status(baseURL, token string) (*UpdateStatus, error) {
	var r targetStatusView
	if err := requestJSON(a.http, http.MethodGet, joinURL(baseURL, "/api/v1/admin/update"), token, &r); err != nil {
		return nil, err
	}
	return &UpdateStatus{
		Available:      r.Available,
		Busy:           r.Busy,
		Phase:          r.Phase,
		Lines:          r.Lines,
		Error:          r.Error,
		CurrentVersion: r.Version,
	}, nil
}

// Start 触发目标在线更新（POST /api/v1/admin/update）。
// sso 返回 202 Accepted + 状态快照（无 started 字段），2xx 即视为已受理，进度靠 Status 轮询。
func (a *ModernchatSSO) Start(baseURL, token string) error {
	return requestJSON(a.http, http.MethodPost, joinURL(baseURL, "/api/v1/admin/update"), token, nil)
}
