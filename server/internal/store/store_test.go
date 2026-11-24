package store

import (
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// newTestDB 连接 DB_DSN 指定的 MySQL；不可用则跳过（CI 已配置 MySQL 服务）。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN 未设置，跳过存储层测试（CI 已配置 MySQL 服务）")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Skipf("数据库连接失败，跳过存储层测试: %v", err)
	}
	if err := db.AutoMigrate(&model.OAuthClient{}, &model.OAuthCode{}, &model.OAuthToken{}, &model.User{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	cleanup := func() {
		db.Exec("DELETE FROM o_auth_tokens")
		db.Exec("DELETE FROM o_auth_codes")
		db.Exec("DELETE FROM o_auth_clients")
	}
	cleanup()
	t.Cleanup(cleanup)
	return db
}

// TestOAuthTakeCodeAtomic 授权码必须原子核销：并发兑换同一码只能成功一次，防重放。
func TestOAuthTakeCodeAtomic(t *testing.T) {
	db := newTestDB(t)
	s := NewOAuthStore(db)
	hash := "test-code-hash-atomic-0123456789abcdef"
	now := time.Now()
	if err := s.SaveCode(&model.OAuthCode{
		CodeHash: hash, ClientID: "c1", UserID: 1, RedirectURI: "https://example.com/cb",
		Scope: "profile", ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}

	const n = 8
	var wg sync.WaitGroup
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.TakeCode(hash, "c1", "https://example.com/cb", now)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("并发兑换应恰好成功 1 次，got %d", ok)
	}
}

// TestOAuthTakeCodeRejectsMismatch 授权码与 client/redirect 绑定，错配不可兑换。
func TestOAuthTakeCodeRejectsMismatch(t *testing.T) {
	db := newTestDB(t)
	s := NewOAuthStore(db)
	hash := "test-code-hash-mismatch-0123456789abcd"
	now := time.Now()
	if err := s.SaveCode(&model.OAuthCode{
		CodeHash: hash, ClientID: "c1", UserID: 1, RedirectURI: "https://example.com/cb",
		Scope: "profile", ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}

	for _, tc := range []struct{ client, redirect string }{
		{"other", "https://example.com/cb"},
		{"c1", "https://evil.example.com/cb"},
	} {
		if _, err := s.TakeCode(hash, tc.client, tc.redirect, now); err == nil {
			t.Fatalf("错配应拒绝: client=%s redirect=%s", tc.client, tc.redirect)
		}
	}
	// 正确组合仍可兑换（错配尝试不应消耗码）
	if _, err := s.TakeCode(hash, "c1", "https://example.com/cb", now); err != nil {
		t.Fatalf("错配尝试不应消耗码: %v", err)
	}
}
