package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/adapter"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/service"
)

// InstanceManage 消费方定义的窄接口（由 service.InstanceService 实现）。
type InstanceManage interface {
	List() ([]model.ServiceInstance, error)
	Create(kind, name, baseURL, authType, token, healthPath, versionPath string, enabled bool, note string) (*model.ServiceInstance, error)
	Get(id uint) (*model.ServiceInstance, error)
	Update(id uint, kind, name, baseURL, authType, token, healthPath, versionPath string, enabled bool, note string) (*model.ServiceInstance, error)
	Delete(id uint) error
	Probe(id uint) (*model.ServiceInstance, error)
	ProbeAll() ([]service.ProbeAllResult, error)
}

// InstanceUpdater 消费方定义的窄接口：更新编排代理（由 service.InstanceService 实现）。
type InstanceUpdater interface {
	UpdateCheck(id uint) (*adapter.UpdateCheck, error)
	UpdateStatus(id uint) (*adapter.UpdateStatus, error)
	UpdateStart(id uint) error
}

// InstanceAdmin 消费方定义的窄接口：管理操作代理（统一管理后台，由 service.InstanceService 实现）。
type InstanceAdmin interface {
	AdminStats(id uint) (*adapter.AdminStats, error)
	AdminUsers(id uint, page, pageSize int, kw, role, status string) (*adapter.AdminUserPage, error)
	AdminBan(id, targetUserID uint, days int, reason string) error
	AdminUnban(id, targetUserID uint) error
	AdminKick(id, targetUserID uint) (bool, error)
	AdminResetPassword(id, targetUserID uint) (string, error)
}

// InstanceHandler 服务实例端点（RESTful：/api/v1/services + update/admin 子资源）。
type InstanceHandler struct {
	instances InstanceManage
	updates   InstanceUpdater
	admins    InstanceAdmin
}

// NewInstanceHandler 构造。
func NewInstanceHandler(instances InstanceManage, updates InstanceUpdater, admins InstanceAdmin) *InstanceHandler {
	return &InstanceHandler{instances: instances, updates: updates, admins: admins}
}

type upsertReq struct {
	Kind        string `json:"kind"`
	Name        string `json:"name" binding:"required"`
	BaseURL     string `json:"baseUrl" binding:"required"`
	AuthType    string `json:"authType"`
	Token       string `json:"token"`
	HealthPath  string `json:"healthPath"`
	VersionPath string `json:"versionPath"`
	Enabled     *bool  `json:"enabled"` // 缺省 true
	Note        string `json:"note"`
}

// List 服务实例列表。
func (h *InstanceHandler) List(c *gin.Context) {
	list, err := h.instances.List()
	if err != nil {
		mapErr(c, err)
		return
	}
	if list == nil {
		list = []model.ServiceInstance{} // 空列表返回 [] 而不是 null
	}
	c.JSON(http.StatusOK, list)
}

// Create 注册实例（201）。
func (h *InstanceHandler) Create(c *gin.Context) {
	var req upsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不合法：name 与 baseUrl 必填")
		return
	}
	inst, err := h.instances.Create(req.Kind, req.Name, req.BaseURL, orNone(req.AuthType), req.Token, req.HealthPath, req.VersionPath, orTrue(req.Enabled), req.Note)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, inst)
}

// Get 单个实例。
func (h *InstanceHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	inst, err := h.instances.Get(id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, inst)
}

// Update 编辑实例（token 空值=保持原凭据不变）。
func (h *InstanceHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req upsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不合法：name 与 baseUrl 必填")
		return
	}
	inst, err := h.instances.Update(id, req.Kind, req.Name, req.BaseURL, orNone(req.AuthType), req.Token, req.HealthPath, req.VersionPath, orTrue(req.Enabled), req.Note)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, inst)
}

// Delete 删除实例（204）。
func (h *InstanceHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.instances.Delete(id); err != nil {
		mapErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Check 触发一次健康/版本探测，返回更新后的实例。
func (h *InstanceHandler) Check(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	inst, err := h.instances.Probe(id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, inst)
}

// probeAllResp 批量探测汇总。
type probeAllResp struct {
	Total   int                      `json:"total"`
	OK      int                      `json:"ok"`
	Fail    int                      `json:"fail"`
	Results []service.ProbeAllResult `json:"results"`
}

// CheckAll 并发探测全部实例（有界并发；概览页一键刷新用）。
func (h *InstanceHandler) CheckAll(c *gin.Context) {
	results, err := h.instances.ProbeAll()
	if err != nil {
		mapErr(c, err)
		return
	}
	resp := probeAllResp{Total: len(results), Results: results}
	for _, r := range results {
		if r.LastStatus == "ok" {
			resp.OK++
		} else {
			resp.Fail++
		}
	}
	c.JSON(http.StatusOK, resp)
}

// ── 更新编排代理（/services/:id/update/*，经 kind 对应 adapter）──

// UpdateCheck 检查目标服务的新版本。
func (h *InstanceHandler) UpdateCheck(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.updates.UpdateCheck(id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// UpdateStatus 查询目标更新任务状态（Web 端轮询）。
func (h *InstanceHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.updates.UpdateStatus(id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// UpdateStart 触发目标在线更新（异步执行，进度靠 UpdateStatus 轮询）。
func (h *InstanceHandler) UpdateStart(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.updates.UpdateStart(id); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"started": true})
}

// parseID 解析路径参数 :id；失败时已写好 400 响应。
func parseID(c *gin.Context) (uint, bool) {
	return parsePositiveID(c, "id")
}

// parseUID 解析路径参数 :uid（目标服务里的用户 id）。
func parseUID(c *gin.Context) (uint, bool) {
	return parsePositiveID(c, "uid")
}

func parsePositiveID(c *gin.Context, name string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || n == 0 {
		slog.Debug("非法路径 ID", "name", name, "raw", c.Param(name))
		fail(c, http.StatusBadRequest, "参数不合法："+name+" 须为正整数")
		return 0, false
	}
	return uint(n), true
}

// pageParams 归一分页参数（page≥1 默认 1；pageSize 1–100 默认 20，与主仓库口径一致）。
func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// ── 管理操作端点（/services/:id/admin/*，经 adminer 代理目标服务站长 API）──

// AdminStats 目标服务概览统计。
func (h *InstanceHandler) AdminStats(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.admins.AdminStats(id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AdminUsers 目标服务用户分页列表（kw/role/status 归一后透传）。
func (h *InstanceHandler) AdminUsers(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	page, pageSize := pageParams(c)
	kw := c.Query("kw")
	if utf8.RuneCountInString(kw) > 50 {
		kw = string([]rune(kw[:50]))
	}
	role := c.Query("role")
	if role != "user" && role != "admin" {
		role = ""
	}
	status := c.Query("status")
	if status != "active" && status != "banned" {
		status = ""
	}
	res, err := h.admins.AdminUsers(id, page, pageSize, kw, role, status)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type adminBanReq struct {
	Days   *int   `json:"days"` // 缺省 7 天；0=永久
	Reason string `json:"reason"`
}

// AdminBan 封禁目标用户。
func (h *InstanceHandler) AdminBan(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	uid, ok := parseUID(c)
	if !ok {
		return
	}
	var req adminBanReq
	_ = c.ShouldBindJSON(&req) // body 可省略，走默认值
	days := 7
	if req.Days != nil {
		if *req.Days < 0 || *req.Days > 3650 {
			fail(c, http.StatusBadRequest, "参数不合法：days 须为 0–3650（0=永久）")
			return
		}
		days = *req.Days
	}
	reason := strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(reason) > 200 {
		fail(c, http.StatusBadRequest, "参数不合法：reason 最多 200 字")
		return
	}
	if err := h.admins.AdminBan(id, uid, days, reason); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminUnban 解封目标用户。
func (h *InstanceHandler) AdminUnban(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	uid, ok := parseUID(c)
	if !ok {
		return
	}
	if err := h.admins.AdminUnban(id, uid); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminKick 强制下线目标用户（不封禁）。
func (h *InstanceHandler) AdminKick(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	uid, ok := parseUID(c)
	if !ok {
		return
	}
	wasOnline, err := h.admins.AdminKick(id, uid)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "wasOnline": wasOnline})
}

// AdminResetPassword 目标端随机重置密码（一次性明文只在本响应出现，不落库不进日志）。
func (h *InstanceHandler) AdminResetPassword(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	uid, ok := parseUID(c)
	if !ok {
		return
	}
	newPassword, err := h.admins.AdminResetPassword(id, uid)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"newPassword": newPassword})
}

// orTrue enabled 缺省 true。
func orTrue(p *bool) bool { return p == nil || *p }

// orNone authType 缺省 none。
func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
