// Package router 组合根：装配 store → service → handler/middleware。
// 依赖方向单向：handler/middleware →（窄接口）→ service → store；本包是唯一知道全部具体类型的地方。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/config"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/handler"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/middleware"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/service"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/store"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/version"
)

// Setup 装配全部路由。
func Setup(cfg config.Config, users *store.UserStore, instances *store.InstanceStore, box *crypto.Box) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// 版本端点免鉴权（与主仓库同口径），Hub 自身探测与运维核对都用它
	r.GET("/api/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, version.Current())
	})

	accounts := service.NewAccountService(users, cfg.JWTSecret)
	authH := handler.NewAuthHandler(accounts, cfg.CookieSecure)
	authMw := middleware.Auth(accounts) // *service.AccountService 满足窄接口 Authenticator

	instancesSvc := service.NewInstanceService(instances, box)
	instH := handler.NewInstanceHandler(instancesSvc)

	v1 := r.Group("/api/v1")
	// TODO(限流)：注册/登录挂 IP 级限流 + 账号级失败锁定（参照主仓库 middleware/limiter.go）；
	// 落地时做成独立中间件，经窄接口注入，不侵入 handler/service
	v1.POST("/users", authH.Register) // 注册
	v1.POST("/sessions", authH.Login) // 登录（创建会话）

	authed := v1.Group("", authMw)
	authed.GET("/users/me", authH.Me)                // 当前用户
	authed.DELETE("/sessions/current", authH.Logout) // 登出（吊销全部旧 token）

	// 服务实例管理：面板是管理后台，实例 CRUD 只对管理员开放
	admin := authed.Group("", middleware.Admin())
	admin.GET("/services", instH.List)                     // 列表
	admin.POST("/services", instH.Create)                  // 注册实例
	admin.GET("/services/:id", instH.Get)                  // 详情
	admin.PUT("/services/:id", instH.Update)               // 编辑（token 空值=保持不变）
	admin.DELETE("/services/:id", instH.Delete)            // 删除
	admin.POST("/services/:id/actions/check", instH.Check) // 立即探测

	return r
}
