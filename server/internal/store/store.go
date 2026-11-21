// Package store 数据访问层：只写查询与少量业务不变量，不做参数校验。
package store

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// Open 建立 MySQL 连接；autoMigrate=true 时启动期建表（开发环境姿势，
// 生产改走 server/migrations/ 版本化 SQL + golang-migrate，MIGRATE=true）。
// 连接池口径与主仓库一致（200/50/1h）：上限兜底突发，空闲常驻减少握手，
// 连接最长存活 1 小时避免 MySQL wait_timeout 单方面断连后的坏连接。
func Open(dsn string, autoMigrate bool) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		// 唯一键冲突等方言错误转为 gorm.ErrDuplicatedKey，service 才能映射 409 而非 500
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if autoMigrate {
		if err := db.AutoMigrate(&model.User{}, &model.ServiceInstance{}, &model.LoginAttempt{}); err != nil {
			return nil, err
		}
	}
	return db, nil
}
