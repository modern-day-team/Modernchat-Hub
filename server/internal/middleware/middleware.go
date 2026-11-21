// Package middleware HTTP 中间件：只做传输层认证（提取凭证 → 委托校验 → 状态码）。
// 业务校验经窄接口 Authenticator 委托给 service；本包不 import store，不写业务规则。
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/token"
)

// Authenticator 消费方定义的窄接口：把原始凭证解析为用户（由 service.AccountService 实现）。
type Authenticator interface {
	Authenticate(rawToken string) (*model.User, error)
}

// Auth 认证中间件：Authorization: Bearer 或 mc_auth cookie 二选一。
func Auth(auth Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			if ck, err := c.Cookie(token.AuthCookieName); err == nil {
				raw = ck
			}
		}
		if raw != "" {
			u, err := auth.Authenticate(raw)
			switch {
			case err == nil:
				c.Set("user", u)
				c.Next()
				return
			case errors.Is(err, apperr.ErrBanned):
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			case errors.Is(err, apperr.ErrInvalidToken):
				// 落到统一的 401
			default:
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "服务器开小差了，请稍后再试"})
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": apperr.ErrInvalidToken.Error()})
	}
}

// UserFrom 取当前用户（Auth 之后可用）。
func UserFrom(c *gin.Context) *model.User {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	u, _ := v.(*model.User)
	return u
}

// Admin 管理员中间件：挂在 Auth 之后，role!=admin 一律 403（姿势与主仓库一致）。
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := UserFrom(c); u == nil || u.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
