package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// InstanceManage 消费方定义的窄接口（由 service.InstanceService 实现）。
type InstanceManage interface {
	List() ([]model.ServiceInstance, error)
	Create(name, baseURL, authType, token, healthPath string, enabled bool, note string) (*model.ServiceInstance, error)
	Get(id uint) (*model.ServiceInstance, error)
	Update(id uint, name, baseURL, authType, token, healthPath string, enabled bool, note string) (*model.ServiceInstance, error)
	Delete(id uint) error
	Probe(id uint) (*model.ServiceInstance, error)
}

// InstanceHandler 服务实例端点（RESTful：/api/v1/services）。
type InstanceHandler struct {
	instances InstanceManage
}

// NewInstanceHandler 构造。
func NewInstanceHandler(instances InstanceManage) *InstanceHandler {
	return &InstanceHandler{instances: instances}
}

type upsertReq struct {
	Name       string `json:"name" binding:"required"`
	BaseURL    string `json:"baseUrl" binding:"required"`
	AuthType   string `json:"authType"`
	Token      string `json:"token"`
	HealthPath string `json:"healthPath"`
	Enabled    *bool  `json:"enabled"` // 缺省 true
	Note       string `json:"note"`
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
	inst, err := h.instances.Create(req.Name, req.BaseURL, orNone(req.AuthType), req.Token, req.HealthPath, orTrue(req.Enabled), req.Note)
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
	inst, err := h.instances.Update(id, req.Name, req.BaseURL, orNone(req.AuthType), req.Token, req.HealthPath, orTrue(req.Enabled), req.Note)
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

// parseID 解析路径参数 :id；失败时已写好 400 响应。
func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		slog.Debug("非法实例 ID", "raw", c.Param("id"))
		fail(c, http.StatusBadRequest, "参数不合法：id 须为正整数")
		return 0, false
	}
	return uint(n), true
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
