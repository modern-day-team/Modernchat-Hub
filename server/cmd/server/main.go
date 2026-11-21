// Modernchat-Hub：Modernchat 项目组统一管理面板入口。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/config"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/crypto"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/router"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}
	// release 模式关闭 gin 的调试告警与路由打印，性能与日志噪音双收益（GIN_MODE=debug 可切回）
	gin.SetMode(cfg.GinMode)
	db, err := store.Open(cfg.DBDSN, true)
	if err != nil {
		slog.Error("数据库连接失败（本地开发先在仓库根目录 docker compose up -d）", "err", err)
		os.Exit(1)
	}

	// 服务凭据加解密器：CRED_SECRET 未配置时由 JWT_SECRET 独立前缀派生
	box := crypto.NewBox(cfg.CredSecret, cfg.JWTSecret)

	// 只设读头/空闲超时，不设 ReadTimeout / WriteTimeout（与主仓库同口径）：
	// 未来 WebSocket 与文件上传/下载都是长连接，整体读写超时会误杀正常会话。
	// ReadHeaderTimeout 防 Slowloris（只发一半请求头长期占连接），
	// IdleTimeout 回收 keep-alive 空闲连接（默认无上限）。
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.Setup(cfg, store.NewUserStore(db), store.NewInstanceStore(db), store.NewLoginAttemptStore(db), box),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		slog.Info("Hub 服务启动", "port", cfg.Port, "mode", cfg.GinMode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("收到退出信号，正在优雅关机……")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("关机超时", "err", err)
	}
	slog.Info("已退出")
}
