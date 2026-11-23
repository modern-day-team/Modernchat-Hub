// Package handler 的 OAuth Provider 传输层：只做绑定/响应组装/错误映射，业务在 service。
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/middleware"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/service"
)

// OAuthManage 客户端注册窄接口（由 service.OAuthService 实现）。
type OAuthManage interface {
	RegisterClient(clientID, redirectURI, scopes string) (string, error)
}

// OAuthHandler OAuth Provider 端点。
type OAuthHandler struct {
	oauth *service.OAuthService
}

func NewOAuthHandler(oauth *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauth: oauth}
}

// Authorize GET /oauth/authorize?client_id=&redirect_uri=&state=&scope=
// 需登录态（authMw 挂在路由上）；state 由主仓库生成，这里原样回传、不解析。
func (h *OAuthHandler) Authorize(c *gin.Context) {
	u := middleware.UserFrom(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录 Hub 账号"})
		return
	}
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	scope := c.Query("scope")
	if clientID == "" || redirectURI == "" || state == "" {
		fail(c, http.StatusBadRequest, "参数不合法：client_id、redirect_uri、state 必填")
		return
	}
	code, err := h.oauth.Authorize(clientID, redirectURI, scope, u.ID)
	if err != nil {
		mapOAuthErr(c, err)
		return
	}
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	c.Redirect(http.StatusFound, redirectURI+sep+"code="+code+"&state="+state)
}

// Token POST /api/oauth/token —— 请求体 JSON，无 grant_type（主仓库客户端口径）。
func (h *OAuthHandler) Token(c *gin.Context) {
	var req struct {
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
		Code         string `json:"code" binding:"required"`
		RedirectURI  string `json:"redirect_uri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不合法：client_id、client_secret、code、redirect_uri 必填")
		return
	}
	token, err := h.oauth.Exchange(req.ClientID, req.ClientSecret, req.Code, req.RedirectURI)
	if err != nil {
		// token 端点错误也返回可解析 JSON（主仓库只读 access_token / error 两字段）
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": 86400})
}

// UserInfo GET /api/oauth/userinfo —— Bearer access_token。
func (h *OAuthHandler) UserInfo(c *gin.Context) {
	raw := ""
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		raw = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if raw == "" {
		c.JSON(http.StatusOK, gin.H{"error": "缺少访问令牌"})
		return
	}
	info, err := h.oauth.UserInfo(raw)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

// RegisterClient 管理员注册 OAuth 客户端（返回一次性明文 secret）。
func (h *OAuthHandler) RegisterClient(c *gin.Context) {
	var req struct {
		ClientID    string `json:"clientId" binding:"required"`
		RedirectURI string `json:"redirectUri" binding:"required"`
		Scopes      string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不合法：clientId、redirectUri 必填")
		return
	}
	secret, err := h.oauth.RegisterClient(req.ClientID, req.RedirectURI, req.Scopes)
	if err != nil {
		mapOAuthErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"clientId":     req.ClientID,
		"clientSecret": secret,
		"redirectUri":  req.RedirectURI,
		"note":         "clientSecret 仅此一次返回，请妥善保存",
	})
}

func mapOAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperr.ErrOAuthClient), errors.Is(err, apperr.ErrOAuthRedirect),
		errors.Is(err, apperr.ErrOAuthCode), errors.Is(err, apperr.ErrInvalidInput):
		fail(c, http.StatusBadRequest, err.Error())
	default:
		mapErr(c, err)
	}
}
