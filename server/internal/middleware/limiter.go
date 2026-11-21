// Package middleware HTTP 中间件：只做传输层认证/限流（提取凭证 → 委托校验 → 状态码）。
// 业务校验经窄接口委托给 service；本包不 import store，不写业务规则。
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter 按 IP 的内存令牌桶限流（项目组口径：认证类接口每 IP 10 次/分）。
// Redis 共享配额是水平扩展时的事，本期单实例内存桶足够。
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastSeen map[string]time.Time
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter 构造：perMinute = 每分钟允许的请求数（突发上限同值）。
func NewIPRateLimiter(perMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rate:     rate.Every(time.Minute / time.Duration(perMinute)),
		burst:    perMinute,
	}
}

// Allow 判定该 IP 是否放行本次请求。
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lm, ok := l.limiters[ip]
	if !ok {
		lm = rate.NewLimiter(l.rate, l.burst)
		l.limiters[ip] = lm
	}
	l.lastSeen[ip] = time.Now()
	l.sweepLocked()
	return lm.Allow()
}

// sweepLocked 惰性清理：表过大时丢掉 30 分钟未活跃的 IP（避免长期运行内存膨胀）。
func (l *IPRateLimiter) sweepLocked() {
	if len(l.limiters) <= 4096 {
		return
	}
	cutoff := time.Now().Add(-30 * time.Minute)
	for ip, seen := range l.lastSeen {
		if seen.Before(cutoff) {
			delete(l.limiters, ip)
			delete(l.lastSeen, ip)
		}
	}
}

// Middleware gin 中间件：超限 429，文案与主仓库一致。
func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "操作过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
