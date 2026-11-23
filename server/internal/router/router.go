// Package router 组合根：装配 store → service → handler/middleware。
// 依赖方向单向：handler/middleware →（窄接口）→ service → store；本包是唯一知道全部具体类型的地方。
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/adapter"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/config"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/handler"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/middleware"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/service"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/store"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/version"
)

// Setup 装配全部路由。
func Setup(cfg config.Config, users *store.UserStore, instances *store.InstanceStore, attempts *store.LoginAttemptStore, oauthStore *store.OAuthStore, box *crypto.Box) *gin.Engine {
	// gin.New 而非 Default：模式由 config 统一控制（release 下无 debug 告警），
	// 中间件显式装配（Logger + Recovery）
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 显式限定可信代理（ClientIP 防 X-Forwarded-For 伪造，限流与审计都依赖它）：
	// 默认仅回环；TRUSTED_PROXIES 显式空串 = 不信任任何代理（nil 语义是不使用转发头）。
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		slog.Warn("TRUSTED_PROXIES 配置非法，退化为不信任任何代理", "err", err)
		_ = r.SetTrustedProxies(nil)
	}

	// REST 响应压缩（与主仓库同口径）：
	// 未来 /ws 必须排除——WebSocket 升级依赖 Hijack，被 gzip 包装的 ResponseWriter 会破坏握手；
	// MinLength=1024：小于该长度的响应压缩后反而更大（gzip 头 + 熵不足）。
	r.Use(gzip.Gzip(
		gzip.DefaultCompression,
		gzip.WithExcludedPaths([]string{"/ws"}),
		gzip.WithMinLength(1024),
	))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// 版本端点免鉴权（与主仓库同口径），Hub 自身探测与运维核对都用它
	r.GET("/api/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, version.Current())
	})

	accounts := service.NewAccountService(users, attempts, cfg.LoginMaxAttempts, cfg.LoginLockMinutes, cfg.JWTSecret)
	authH := handler.NewAuthHandler(accounts, cfg.CookieSecure)
	authMw := middleware.Auth(accounts) // *service.AccountService 满足窄接口 Authenticator

	// OAuth Provider：Hub 作为项目组统一登录中心，给主仓库等客户端发授权码/令牌。
	// 契约对齐主仓库 SSO 客户端：/oauth/authorize + /api/oauth/{token,userinfo}。
	oauthSvc := service.NewOAuthService(oauthStore)
	oauthH := handler.NewOAuthHandler(oauthSvc)
	r.GET("/oauth/authorize", authMw, oauthH.Authorize)
	op := r.Group("/api/oauth")
	op.POST("/token", oauthH.Token)
	op.GET("/userinfo", oauthH.UserInfo)

	// adapter 注册表：项目组特化能力在此装配；内核不感知具体项目组服务
	updates := adapter.NewRegistry()
	mcn := adapter.NewModernchatNew()
	updates.RegisterUpdater(mcn)
	updates.RegisterAdmin(mcn)
	instancesSvc := service.NewInstanceService(instances, box, updates)
	instH := handler.NewInstanceHandler(instancesSvc, instancesSvc, instancesSvc)

	v1 := r.Group("/api/v1")
	// 认证类接口 IP 级限流（项目组口径：每 IP 每路由 10 次/分，内存令牌桶，按路由独立配额）；
	// 账号级失败锁定在 AccountService.Login 内（login_attempts 表，按 username 计）
	registerLimiter := middleware.NewIPRateLimiter(10).Middleware()
	loginLimiter := middleware.NewIPRateLimiter(10).Middleware()
	v1.POST("/users", registerLimiter, authH.Register) // 注册
	v1.POST("/sessions", loginLimiter, authH.Login)    // 登录（创建会话）

	authed := v1.Group("", authMw)
	authed.GET("/users/me", authH.Me)                // 当前用户
	authed.DELETE("/sessions/current", authH.Logout) // 登出（吊销全部旧 token）

	// 服务实例管理：面板是管理后台，实例 CRUD 只对管理员开放
	admin := authed.Group("", middleware.Admin())
	admin.POST("/oauth/clients", oauthH.RegisterClient)       // 注册 OAuth 客户端（返回一次性明文 secret）
	admin.GET("/services", instH.List)                        // 列表
	admin.POST("/services", instH.Create)                     // 注册实例
	admin.GET("/services/:id", instH.Get)                     // 详情
	admin.PUT("/services/:id", instH.Update)                  // 编辑（token 空值=保持不变）
	admin.DELETE("/services/:id", instH.Delete)               // 删除
	admin.POST("/services/:id/actions/check", instH.Check)    // 立即探测
	admin.POST("/services/actions/check-all", instH.CheckAll) // 批量并发探测（集合级操作）

	// 更新编排子资源（经 kind 对应 adapter 代理目标服务；未注册 kind → 400）
	admin.GET("/services/:id/update/check", instH.UpdateCheck)   // 检查新版本
	admin.GET("/services/:id/update/status", instH.UpdateStatus) // 更新任务状态（轮询）
	admin.POST("/services/:id/update/start", instH.UpdateStart)  // 触发在线更新

	// 管理操作子资源（统一管理后台：代理目标服务站长 API；未注册 kind → 400）
	admin.GET("/services/:id/admin/stats", instH.AdminStats)
	admin.GET("/services/:id/admin/users", instH.AdminUsers)
	admin.POST("/services/:id/admin/users/:uid/ban", instH.AdminBan)
	admin.POST("/services/:id/admin/users/:uid/unban", instH.AdminUnban)
	admin.POST("/services/:id/admin/users/:uid/kick", instH.AdminKick)
	admin.POST("/services/:id/admin/users/:uid/reset-password", instH.AdminResetPassword)
	admin.GET("/services/:id/admin/plugins", instH.AdminPlugins)                                 // 插件聚合列表
	admin.POST("/services/:id/admin/plugins/global-disable", instH.AdminSetPluginGlobalDisabled) // 全局下架/恢复
	admin.GET("/services/:id/admin/audits", instH.AdminAudits)                                   // 审计日志
	admin.POST("/services/:id/admin/announce", instH.AdminAnnounce)                              // 全站公告
	admin.GET("/services/:id/admin/settings", instH.AdminSettings)                               // 站点设置读
	admin.PUT("/services/:id/admin/settings", instH.AdminSaveSettings)                           // 站点设置写

	return r
}
