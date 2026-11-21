// Package adapter 家族特化适配层：把「被管理服务」的差异收敛在 adapter，
// 内核（service）只依赖本包定义的窄接口与归一化结果——
// 新增一个被管理服务 = 注册配置 +（必要时）一个 adapter，不改内核代码（AGENTS.md 通用型设计原则）。
package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
)

// UpdateCheck 更新检查结果（已归一，跨服务口径一致）。
type UpdateCheck struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// UpdateStatus 目标服务更新任务的运行状态快照（尽力归一）。
type UpdateStatus struct {
	Available      bool     `json:"available"`                // 目标是否支持在线更新
	Busy           bool     `json:"busy"`                     // 是否有更新任务进行中
	Phase          string   `json:"phase,omitempty"`          // 当前阶段
	Lines          []string `json:"lines,omitempty"`          // 最近日志行
	Error          string   `json:"error,omitempty"`          // 上次更新错误
	CurrentVersion string   `json:"currentVersion,omitempty"` // 目标当前运行版本
	Mode           string   `json:"mode,omitempty"`           // release | source
}

// Updater 更新能力窄接口：adapter 按目标服务实现，内核经 Registry 消费。
// token 是已解密的目标服务管理员凭据（解密由内核负责，adapter 不接触密文）。
type Updater interface {
	// Kind 返回适配的服务类型标识（与 ServiceInstance.Kind 对应）。
	Kind() string
	Check(baseURL, token string) (*UpdateCheck, error)
	Status(baseURL, token string) (*UpdateStatus, error)
	Start(baseURL, token string) error
}

// Registry 服务类型 → 更新 adapter 的注册表（组合根是唯一装配点）。
type Registry struct {
	mu     sync.RWMutex
	byKind map[string]Updater
}

// NewRegistry 构造空注册表。
func NewRegistry() *Registry {
	return &Registry{byKind: map[string]Updater{}}
}

// Register 注册 adapter（同 kind 后注册者覆盖）。
func (r *Registry) Register(u Updater) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKind[u.Kind()] = u
}

// Updater 按 kind 取 adapter。
func (r *Registry) Updater(kind string) (Updater, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byKind[kind]
	return u, ok
}

// requestJSON 发起带可选 Bearer 凭据的请求并把 2xx 响应解析进 out（out 传 nil 表示忽略响应体）。
func requestJSON(client *http.Client, method, rawURL, token string, out any) error {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return apperr.ErrTargetUnreachable
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w：%s", apperr.ErrTargetUnreachable, err)
	}
	return decodeResponse(resp, out)
}

// decodeResponse 目标服务响应归一：2xx 解析 JSON；其余按状态码映射语义哨兵，
// 目标返回的 {"error":"<中文>"} 原因透传给最终用户。
func decodeResponse(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w：读取响应失败", apperr.ErrTargetUnreachable)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("%w：响应不是合法 JSON", apperr.ErrTargetUnreachable)
		}
		return nil
	}
	msg := strings.TrimSpace(string(body))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		msg = e.Error
	}
	if msg == "" {
		msg = resp.Status
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w：%s", apperr.ErrTargetAuth, msg)
	case http.StatusNotFound:
		return fmt.Errorf("%w：目标未提供该接口（%s）", apperr.ErrUpdateUnsupported, msg)
	case http.StatusConflict:
		return fmt.Errorf("%w：%s", apperr.ErrUpdateBusy, msg)
	case http.StatusBadRequest:
		// 主仓库用 400 表达「部署方式不支持在线更新」与检查失败，原因透传
		return fmt.Errorf("%w：%s", apperr.ErrUpdateUnsupported, msg)
	default:
		return fmt.Errorf("%w：HTTP %d %s", apperr.ErrTargetUnreachable, resp.StatusCode, msg)
	}
}

// joinURL 拼接 base 与路径（容忍 base 末尾多余斜杠）。
func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}
