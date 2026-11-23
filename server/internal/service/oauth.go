// Package service 的 OAuth Provider 业务规则：授权码发放/核销、令牌签发与用户资料。
// 契约严格对齐主仓库 SSO 客户端（web/src/... / server/internal/handler/sso.go）：
//   - /oauth/authorize 原样回传 state，不解析
//   - /api/oauth/token 请求体为 JSON 且无 grant_type
//   - /api/oauth/userinfo 返回 sub/username/nickname/email/avatar
package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/apperr"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
	"github.com/modern-day-team/Modernchat-Hub/server/internal/store"
)

const (
	oauthCodeTTL  = 5 * time.Minute
	oauthTokenTTL = 24 * time.Hour
)

// oauthStore 消费方定义的窄接口（由 store.OAuthStore 实现），单测可用假仓储驱动全部业务分支。
type oauthStore interface {
	Client(clientID string) (*model.OAuthClient, error)
	CreateClient(c *model.OAuthClient) error
	SaveCode(c *model.OAuthCode) error
	TakeCode(codeHash, clientID, redirectURI string, now time.Time) (*model.OAuthCode, error)
	SaveToken(t *model.OAuthToken) error
	TokenByHash(tokenHash string, now time.Time) (*model.OAuthToken, error)
	UserByID(id uint) (*model.User, error)
}

// OAuthService OAuth Provider 业务规则唯一归属。
type OAuthService struct {
	store oauthStore
}

func NewOAuthService(s *store.OAuthStore) *OAuthService { return &OAuthService{store: s} }

// newOAuthServiceWithStore 仅测试用：注入假仓储。
func newOAuthServiceWithStore(s oauthStore) *OAuthService { return &OAuthService{store: s} }

// UserInfoView userinfo 响应（字段与主仓库 SSO 客户端解析口径一致）。
type UserInfoView struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

func hash256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// RegisterClient 注册 OAuth 客户端（返回明文 secret，仅此一次；库里只存 hash）。
func (s *OAuthService) RegisterClient(clientID, redirectURI, scopes string) (string, error) {
	if clientID == "" || len(clientID) > 64 || redirectURI == "" || len(redirectURI) > 255 {
		return "", apperr.ErrInvalidInput
	}
	if scopes == "" {
		scopes = "profile"
	}
	secret, err := randomToken(32)
	if err != nil {
		return "", err
	}
	err = s.store.CreateClient(&model.OAuthClient{
		ClientID:    clientID,
		SecretHash:  hash256(secret),
		RedirectURI: redirectURI,
		Scopes:      scopes,
		Enabled:     true,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return "", apperr.ErrOAuthClient
		}
		return "", err
	}
	return secret, nil
}

// Authorize 校验客户端与回调地址，发放一次性授权码（state 由 handler 原样回传）。
func (s *OAuthService) Authorize(clientID, redirectURI, scope string, userID uint) (string, error) {
	c, err := s.store.Client(clientID)
	if err != nil {
		if store.IsNotFound(err) {
			return "", apperr.ErrOAuthClient
		}
		return "", err
	}
	if !c.Enabled || c.RedirectURI != redirectURI {
		return "", apperr.ErrOAuthRedirect
	}
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if scope == "" {
		scope = c.Scopes
	}
	err = s.store.SaveCode(&model.OAuthCode{
		CodeHash:    hash256(code),
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   time.Now().Add(oauthCodeTTL),
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

// Exchange 核销授权码并签发访问令牌（请求体 JSON，无 grant_type）。
func (s *OAuthService) Exchange(clientID, secret, code, redirectURI string) (string, error) {
	c, err := s.store.Client(clientID)
	if err != nil {
		if store.IsNotFound(err) {
			return "", apperr.ErrOAuthClient
		}
		return "", err
	}
	if !c.Enabled || c.SecretHash != hash256(secret) {
		return "", apperr.ErrOAuthClient
	}
	cc, err := s.store.TakeCode(hash256(code), clientID, redirectURI, time.Now())
	if err != nil {
		if store.IsNotFound(err) {
			return "", apperr.ErrOAuthCode
		}
		return "", err
	}
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	err = s.store.SaveToken(&model.OAuthToken{
		TokenHash: hash256(token),
		UserID:    cc.UserID,
		ClientID:  clientID,
		Scope:     cc.Scope,
		ExpiresAt: time.Now().Add(oauthTokenTTL),
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserInfo 按访问令牌取用户资料。
func (s *OAuthService) UserInfo(token string) (*UserInfoView, error) {
	t, err := s.store.TokenByHash(hash256(token), time.Now())
	if err != nil {
		if store.IsNotFound(err) {
			return nil, apperr.ErrOAuthToken
		}
		return nil, err
	}
	u, err := s.store.UserByID(t.UserID)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, apperr.ErrOAuthToken
		}
		return nil, err
	}
	if u.IsBanned() {
		return nil, apperr.ErrOAuthToken
	}
	out := &UserInfoView{
		Sub:      uintToString(u.ID),
		Username: u.Username,
		Nickname: u.Nickname,
	}
	if u.Email != nil {
		out.Email = *u.Email
	}
	return out, nil
}

func uintToString(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
