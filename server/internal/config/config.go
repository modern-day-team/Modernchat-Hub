// Package config 配置加载：.env 可选，环境变量优先。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config 服务运行配置。
type Config struct {
	Port         string // HTTP 监听端口
	DBDSN        string // MySQL 连接串
	JWTSecret    string // JWT 签名密钥
	CookieSecure bool   // mc_auth cookie 是否带 Secure（生产必须 true）
	CredSecret   string // 服务凭据加密密钥；可选，缺省由 JWT_SECRET 派生（mc-hub-cred-v1: 前缀）
	GinMode      string // gin 运行模式：release（默认，生产口径）| debug | test

	// 可信反向代理：ClientIP 的取值来源。默认仅回环；显式设为空串 = 不信任任何代理
	// （不使用 X-Forwarded-For，ClientIP 只取 RemoteAddr）——防伪造 IP 绕过限流。
	TrustedProxies []string

	// 登录失败锁定（security.loginMaxAttempts / loginLockMinutes 的项目组口径）：
	// 按 username 计（用户不存在也计数），0 = 关闭锁定。
	LoginMaxAttempts int // 0（关闭）或 2–20，默认 5
	LoginLockMinutes int // 1–1440，默认 15
}

// Load 读取配置；.env 不存在时静默跳过（生产用环境变量注入）。
func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{
		Port:         getEnv("PORT", "8091"),
		DBDSN:        getEnv("DB_DSN", "root:modernchat_hub_dev@tcp(127.0.0.1:3308)/modernchat_hub?charset=utf8mb4&parseTime=true&loc=Local"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		CookieSecure: getEnv("COOKIE_SECURE", "false") == "true",
		CredSecret:   os.Getenv("CRED_SECRET"),
		GinMode:      getEnv("GIN_MODE", "release"),
	}
	cfg.TrustedProxies = parseTrustedProxies()
	cfg.LoginMaxAttempts, cfg.LoginLockMinutes = parseLoginLock()
	// 不提供不安全的默认值：宁可起不来，也不能用空/弱密钥签 token。
	// ≥32 字节与主仓库生产要求一致（运维手册 §JWT_SECRET）
	if len(cfg.JWTSecret) < 32 {
		return cfg, fmt.Errorf("JWT_SECRET 未设置或不足 32 字节（生成：openssl rand -hex 32）")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseTrustedProxies TRUSTED_PROXIES：逗号分隔网段。
// 默认仅回环；显式设为空串 = 不信任任何代理（此时不使用 X-Forwarded-For）。
func parseTrustedProxies() []string {
	raw := os.Getenv("TRUSTED_PROXIES")
	if _, ok := os.LookupEnv("TRUSTED_PROXIES"); ok && raw == "" {
		return nil // 显式空串：不信任任何代理
	}
	if raw == "" {
		raw = "127.0.0.1,::1"
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLoginLock LOGIN_MAX_ATTEMPTS（0=关闭，否则 2–20，默认 5）与 LOGIN_LOCK_MINUTES（1–1440，默认 15）。
func parseLoginLock() (int, int) {
	// 必须区分"未设置/解析失败"（回退默认 5）与"显式 0"（关闭锁定）：
	// Atoi("") 返回 0 且 err != nil，忽略错误会把默认锁定静默关掉。
	raw := os.Getenv("LOGIN_MAX_ATTEMPTS")
	n, err := strconv.Atoi(raw)
	maxAttempts := 5
	if err == nil {
		if n == 0 {
			maxAttempts = 0 // 显式 0 = 关闭锁定
		} else if n >= 2 && n <= 20 {
			maxAttempts = n
		}
	}
	minutes, _ := strconv.Atoi(getEnv("LOGIN_LOCK_MINUTES", "15"))
	if minutes < 1 || minutes > 1440 {
		minutes = 15
	}
	return maxAttempts, minutes
}
