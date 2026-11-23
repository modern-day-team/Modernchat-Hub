package service

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// fakeOAuthStore 内存假仓储：覆盖 OAuth 全流程分支。
type fakeOAuthStore struct {
	client    *model.OAuthClient
	codes     map[string]*model.OAuthCode
	tokens    map[string]*model.OAuthToken
	user      *model.User
	getErr    error
	usedCodes map[string]bool
}

func newFakeOAuthStore() *fakeOAuthStore {
	return &fakeOAuthStore{
		client: &model.OAuthClient{
			ClientID: "mcn", SecretHash: "s-secret", RedirectURI: "https://chat.example.com/api/auth/sso/callback",
			Scopes: "profile", Enabled: true,
		},
		codes: map[string]*model.OAuthCode{}, tokens: map[string]*model.OAuthToken{},
		user:      &model.User{ID: 7, Username: "alice", Nickname: "Alice"},
		usedCodes: map[string]bool{},
	}
}

func (f *fakeOAuthStore) Client(id string) (*model.OAuthClient, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.client == nil || f.client.ClientID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.client, nil
}
func (f *fakeOAuthStore) CreateClient(c *model.OAuthClient) error {
	if f.client != nil && f.client.ClientID == c.ClientID {
		return gorm.ErrDuplicatedKey
	}
	f.client = c
	return nil
}
func (f *fakeOAuthStore) SaveCode(c *model.OAuthCode) error {
	f.codes[c.CodeHash] = c
	return nil
}
func (f *fakeOAuthStore) TakeCode(hash, clientID, redirectURI string, now time.Time) (*model.OAuthCode, error) {
	c := f.codes[hash]
	if c == nil || f.usedCodes[hash] || c.ClientID != clientID || c.RedirectURI != redirectURI || !c.ExpiresAt.After(now) {
		return nil, gorm.ErrRecordNotFound
	}
	f.usedCodes[hash] = true
	return c, nil
}
func (f *fakeOAuthStore) SaveToken(t *model.OAuthToken) error {
	f.tokens[t.TokenHash] = t
	return nil
}
func (f *fakeOAuthStore) TokenByHash(hash string, now time.Time) (*model.OAuthToken, error) {
	t := f.tokens[hash]
	if t == nil || !t.ExpiresAt.After(now) || t.RevokedAt != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}
func (f *fakeOAuthStore) UserByID(id uint) (*model.User, error) {
	if f.user == nil || f.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.user, nil
}

func TestOAuthFlow(t *testing.T) {
	f := newFakeOAuthStore()
	svc := newOAuthServiceWithStore(f)

	// 注册：明文 secret 仅一次，库里存 hash
	secret, err := svc.RegisterClient("mcn2", "https://a.example.com/cb", "")
	if err != nil || secret == "" {
		t.Fatalf("RegisterClient() = %q, %v", secret, err)
	}
	if f.client.SecretHash == secret {
		t.Fatal("客户端密钥明文不得落库")
	}

	// 恢复主客户端用于后续流程（注册会覆盖 fake 的 client 指针）
	f.client = &model.OAuthClient{
		ClientID: "mcn", SecretHash: hash256("correct-secret"),
		RedirectURI: "https://chat.example.com/api/auth/sso/callback", Scopes: "profile", Enabled: true,
	}

	// 授权码：回调地址必须精确匹配
	code, err := svc.Authorize("mcn", f.client.RedirectURI, "profile", 7)
	if err != nil || code == "" {
		t.Fatalf("Authorize() = %q, %v", code, err)
	}
	if _, err := svc.Authorize("mcn", "https://evil.example.com/cb", "profile", 7); !errors.Is(err, apperr.ErrOAuthRedirect) {
		t.Fatalf("未注册回调地址应拒绝: %v", err)
	}

	// 兑换：secret 错误拒绝；正确则拿到令牌
	if _, err := svc.Exchange("mcn", "wrong", code, f.client.RedirectURI); !errors.Is(err, apperr.ErrOAuthClient) {
		t.Fatalf("错误 secret 应拒绝: %v", err)
	}
	token, err := svc.Exchange("mcn", "correct-secret", code, f.client.RedirectURI)
	if err != nil || token == "" {
		t.Fatalf("Exchange() = %q, %v", token, err)
	}
	// 授权码一次性
	if _, err := svc.Exchange("mcn", "correct-secret", code, f.client.RedirectURI); !errors.Is(err, apperr.ErrOAuthCode) {
		t.Fatalf("授权码应一次性: %v", err)
	}

	// userinfo：字段对齐主仓库客户端
	info, err := svc.UserInfo(token)
	if err != nil {
		t.Fatalf("UserInfo() error: %v", err)
	}
	if info.Sub != "7" || info.Username != "alice" || info.Nickname != "Alice" {
		t.Fatalf("userinfo 字段不符: %+v", info)
	}
	if _, err := svc.UserInfo("bad-token"); !errors.Is(err, apperr.ErrOAuthToken) {
		t.Fatalf("无效令牌应拒绝: %v", err)
	}
}
