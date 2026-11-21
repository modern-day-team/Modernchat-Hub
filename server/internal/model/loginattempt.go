package model

import "time"

// LoginAttempt 登录失败锁定：按 username 计（用户不存在也计数），堵住「用户名不存在」绕过路径；
// 错误文案保持一致，不泄露账号是否存在。凭据正确即删除记录，锁定期过后自动失效。
type LoginAttempt struct {
	Username    string     `gorm:"primaryKey;size:64"`
	FailedCount int        `gorm:"not null;default:0"`
	LockedUntil *time.Time `json:"-"`
	UpdatedAt   time.Time
}
