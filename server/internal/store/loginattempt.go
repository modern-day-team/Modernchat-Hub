package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// LoginAttemptStore login_attempts 表访问：原子计数与锁定（主仓库 login_attempt.go 同语义）。
type LoginAttemptStore struct{ db *gorm.DB }

// NewLoginAttemptStore 构造。
func NewLoginAttemptStore(db *gorm.DB) *LoginAttemptStore { return &LoginAttemptStore{db: db} }

// LockedUntil 该用户名的锁定期截止；无记录或未锁定返回 nil。
func (s *LoginAttemptStore) LockedUntil(username string) (*time.Time, error) {
	var row model.LoginAttempt
	err := s.db.First(&row, "username = ?", username).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return row.LockedUntil, nil
}

// RecordFailure 原子自增失败计数（INSERT ... ON DUPLICATE KEY UPDATE，由数据库保证并发安全）；
// 达到 maxAttempts（>0）时写 locked_until 并清零计数，返回是否触发锁定。
// 计数失败向上返回错误，由 service 记日志降级——计数挂掉不能让登录接口瘫掉。
func (s *LoginAttemptStore) RecordFailure(username string, maxAttempts, lockMinutes int) (bool, error) {
	if maxAttempts <= 0 {
		return false, nil
	}
	now := time.Now()
	if err := s.db.Exec(
		`INSERT INTO login_attempts (username, failed_count, locked_until, updated_at) VALUES (?, 1, NULL, ?)
		 ON DUPLICATE KEY UPDATE failed_count = failed_count + 1, updated_at = ?`,
		username, now, now,
	).Error; err != nil {
		return false, err
	}
	var row model.LoginAttempt
	if err := s.db.First(&row, "username = ?", username).Error; err != nil {
		return false, err
	}
	if row.FailedCount >= maxAttempts {
		until := now.Add(time.Duration(lockMinutes) * time.Minute)
		if err := s.db.Model(&model.LoginAttempt{}).Where("username = ?", username).
			Updates(map[string]any{"locked_until": until, "failed_count": 0}).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// Clear 凭据正确即删除记录（计数清零语义）。
func (s *LoginAttemptStore) Clear(username string) error {
	return s.db.Delete(&model.LoginAttempt{}, "username = ?", username).Error
}
