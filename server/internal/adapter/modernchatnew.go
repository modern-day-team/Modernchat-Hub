package adapter

import (
	"errors"
	"net/http"
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
