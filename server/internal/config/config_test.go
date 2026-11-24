package config

import (
	"os"
	"reflect"
	"testing"
)

// envKeys Load 读取的全部环境变量。未在 set 中指定的键一律 Unset，
// 这样既能避免 godotenv.Load() 注入磁盘 .env，也保留"变量不存在"与"显式空串"的语义差别
// （TRUSTED_PROXIES 显式空串=不信任任何代理，与未设置的默认回环不同）。
var envKeys = []string{
	"PORT", "DB_DSN", "JWT_SECRET", "COOKIE_SECURE", "CRED_SECRET", "GIN_MODE",
	"TRUSTED_PROXIES", "LOGIN_MAX_ATTEMPTS", "LOGIN_LOCK_MINUTES",
}

func isolateEnv(t *testing.T, set map[string]string) {
	t.Helper()
	saved := make(map[string]*string, len(envKeys))
	for _, k := range envKeys {
		if v, ok := os.LookupEnv(k); ok {
			vv := v
			saved[k] = &vv
		} else {
			saved[k] = nil
		}
		_ = os.Unsetenv(k)
	}
	for k, v := range set {
		_ = os.Setenv(k, v)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	})
}

const validSecret = "test-jwt-secret-0123456789abcdef-0123456789abcdef"

// TestLoadRequiresJWTSecret 空或短密钥必须拒绝启动。
func TestLoadRequiresJWTSecret(t *testing.T) {
	isolateEnv(t, map[string]string{"JWT_SECRET": ""})
	if _, err := Load(); err == nil {
		t.Fatal("JWT_SECRET 为空应报错")
	}
	isolateEnv(t, map[string]string{"JWT_SECRET": "short"})
	if _, err := Load(); err == nil {
		t.Fatal("JWT_SECRET 不足 32 字节应报错")
	}
}

// TestLoadDefaults 默认值：release 模式、仅回环可信代理、登录锁定 5 次/15 分钟。
func TestLoadDefaults(t *testing.T) {
	isolateEnv(t, map[string]string{"JWT_SECRET": validSecret})
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != "8091" || c.GinMode != "release" {
		t.Fatalf("默认端口/模式不符: %+v", c)
	}
	if !reflect.DeepEqual(c.TrustedProxies, []string{"127.0.0.1", "::1"}) {
		t.Fatalf("默认可信代理应仅回环，got %v", c.TrustedProxies)
	}
	if c.LoginMaxAttempts != 5 || c.LoginLockMinutes != 15 {
		t.Fatalf("默认锁定参数不符: %d/%d", c.LoginMaxAttempts, c.LoginLockMinutes)
	}
}

// TestLoadTrustedProxies 显式空串 = 不信任任何代理（防 XFF 伪造绕过限流）。
func TestLoadTrustedProxies(t *testing.T) {
	isolateEnv(t, map[string]string{"JWT_SECRET": validSecret, "TRUSTED_PROXIES": ""})
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.TrustedProxies != nil {
		t.Fatalf("显式空串应返回 nil，got %v", c.TrustedProxies)
	}

	isolateEnv(t, map[string]string{
		"JWT_SECRET": validSecret, "TRUSTED_PROXIES": " 10.0.0.1 , 192.168.1.0/24 ,, ",
	})
	c, err = Load()
	if err != nil {
		t.Fatalf("Load custom: %v", err)
	}
	want := []string{"10.0.0.1", "192.168.1.0/24"}
	if !reflect.DeepEqual(c.TrustedProxies, want) {
		t.Fatalf("可信代理解析不符: want=%v got=%v", want, c.TrustedProxies)
	}
}

// TestLoadLoginLock 锁定参数：0=关闭、越界回退默认。
func TestLoadLoginLock(t *testing.T) {
	cases := []struct {
		name     string
		attempts string
		minutes  string
		wantN    int
		wantM    int
	}{
		{"explicit zero", "0", "30", 0, 30},
		{"valid range", "10", "60", 10, 60},
		{"too few", "1", "15", 5, 15},
		{"too many", "21", "15", 5, 15},
		{"minutes too small", "5", "0", 5, 15},
		{"minutes too large", "5", "1441", 5, 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateEnv(t, map[string]string{
				"JWT_SECRET":         validSecret,
				"LOGIN_MAX_ATTEMPTS": tc.attempts,
				"LOGIN_LOCK_MINUTES": tc.minutes,
			})
			c, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if c.LoginMaxAttempts != tc.wantN || c.LoginLockMinutes != tc.wantM {
				t.Fatalf("want=%d/%d got=%d/%d", tc.wantN, tc.wantM, c.LoginMaxAttempts, c.LoginLockMinutes)
			}
		})
	}
}

// TestLoadEnvOverride 其余字段按环境变量覆盖。
func TestLoadEnvOverride(t *testing.T) {
	isolateEnv(t, map[string]string{
		"JWT_SECRET":    validSecret,
		"PORT":          "9001",
		"COOKIE_SECURE": "true",
		"CRED_SECRET":   "independent-cred-secret-0123456789",
		"DB_DSN":        "root:pw@tcp(db:3306)/hub",
	})
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != "9001" || !c.CookieSecure || c.DBDSN != "root:pw@tcp(db:3306)/hub" {
		t.Fatalf("覆盖失败: %+v", c)
	}
	if c.CredSecret != "independent-cred-secret-0123456789" {
		t.Fatalf("独立凭据密钥应保留: %q", c.CredSecret)
	}
}
