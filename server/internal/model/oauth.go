package model

import "time"

// OAuthClient 已注册的 OAuth 客户端（redirect_uri 精确白名单，不支持前缀匹配）。
type OAuthClient struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClientID    string    `gorm:"size:64;uniqueIndex" json:"clientId"`
	SecretHash  string    `gorm:"size:64" json:"-"` // SHA-256(client_secret)，明文不落库
	RedirectURI string    `gorm:"size:255" json:"redirectUri"`
	Scopes      string    `gorm:"size:255;default:profile" json:"scopes"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OAuthCode 一次性授权码（只存哈希；used_at 非空即已消费）。
type OAuthCode struct {
	ID          uint      `gorm:"primaryKey"`
	CodeHash    string    `gorm:"size:64;uniqueIndex"`
	ClientID    string    `gorm:"size:64;index"`
	UserID      uint      `gorm:"index"`
	RedirectURI string    `gorm:"size:255"`
	Scope       string    `gorm:"size:255"`
	ExpiresAt   time.Time `gorm:"index"`
	UsedAt      *time.Time
	CreatedAt   time.Time
}

// OAuthToken 访问令牌（只存哈希，支持撤销；不签发 refresh token）。
type OAuthToken struct {
	ID        uint      `gorm:"primaryKey"`
	TokenHash string    `gorm:"size:64;uniqueIndex"`
	UserID    uint      `gorm:"index"`
	ClientID  string    `gorm:"size:64;index"`
	Scope     string    `gorm:"size:255"`
	ExpiresAt time.Time `gorm:"index"`
	RevokedAt *time.Time
	CreatedAt time.Time
}
