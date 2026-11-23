// Package store 的 OAuth Provider 数据访问：只做查询与原子状态变更，不写业务规则。
package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// OAuthStore OAuth Provider 持久化。
type OAuthStore struct{ db *gorm.DB }

func NewOAuthStore(db *gorm.DB) *OAuthStore { return &OAuthStore{db: db} }

// Client 按 client_id 读取（含停用客户端，由 service 判 enabled）。
func (s *OAuthStore) Client(clientID string) (*model.OAuthClient, error) {
	var c model.OAuthClient
	if err := s.db.Where("client_id = ?", clientID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateClient 注册客户端（密钥 hash 由 service 计算）。
func (s *OAuthStore) CreateClient(c *model.OAuthClient) error {
	return s.db.Create(c).Error
}

// SaveCode 落授权码（hash 由 service 计算）。
func (s *OAuthStore) SaveCode(c *model.OAuthCode) error {
	return s.db.Create(c).Error
}

// TakeCode 原子核销授权码：hash 命中、未使用、未过期才置 used_at。
// 用条件 UPDATE 而非先读后写，避免并发重复兑换。
func (s *OAuthStore) TakeCode(codeHash, clientID, redirectURI string, now time.Time) (*model.OAuthCode, error) {
	var c model.OAuthCode
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"code_hash = ? AND client_id = ? AND redirect_uri = ? AND used_at IS NULL AND expires_at > ?",
			codeHash, clientID, redirectURI, now,
		).First(&c).Error; err != nil {
			return err
		}
		return tx.Model(&model.OAuthCode{}).Where("id = ?", c.ID).
			Update("used_at", now).Error
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveToken 落访问令牌（hash 由 service 计算）。
func (s *OAuthStore) SaveToken(t *model.OAuthToken) error {
	return s.db.Create(t).Error
}

// TokenByHash 按 hash 读取未撤销且未过期的令牌。
func (s *OAuthStore) TokenByHash(tokenHash string, now time.Time) (*model.OAuthToken, error) {
	var t model.OAuthToken
	if err := s.db.Where(
		"token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, now,
	).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// UserByID 供 userinfo 取用户资料。
func (s *OAuthStore) UserByID(id uint) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// IsNotFound 包装 gorm.ErrRecordNotFound 判定。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
