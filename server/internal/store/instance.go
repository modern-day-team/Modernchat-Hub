package store

import (
	"time"

	"gorm.io/gorm"

	"github.com/modern-day-team/Modernchat-Hub/server/internal/model"
)

// InstanceStore service_instances 表访问。
type InstanceStore struct{ db *gorm.DB }

// NewInstanceStore 构造。
func NewInstanceStore(db *gorm.DB) *InstanceStore { return &InstanceStore{db: db} }

// Create 新建实例。
func (s *InstanceStore) Create(i *model.ServiceInstance) error { return s.db.Create(i).Error }

// List 全量列表（按创建时间正序）。
func (s *InstanceStore) List() ([]model.ServiceInstance, error) {
	var out []model.ServiceInstance
	err := s.db.Order("created_at ASC").Find(&out).Error
	return out, err
}

// FindByID 按主键查；不存在返回 gorm.ErrRecordNotFound。
func (s *InstanceStore) FindByID(id uint) (*model.ServiceInstance, error) {
	var i model.ServiceInstance
	if err := s.db.First(&i, id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

// FindByName 按名称查；不存在返回 gorm.ErrRecordNotFound。
func (s *InstanceStore) FindByName(name string) (*model.ServiceInstance, error) {
	var i model.ServiceInstance
	if err := s.db.Where("name = ?", name).First(&i).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

// Save 保存全量编辑。
func (s *InstanceStore) Save(i *model.ServiceInstance) error { return s.db.Save(i).Error }

// Delete 按主键删除。
func (s *InstanceStore) Delete(id uint) error {
	return s.db.Delete(&model.ServiceInstance{}, id).Error
}

// SaveProbeResult 写回一次探测结果（last_version 可为 nil 表示未取到版本）。
func (s *InstanceStore) SaveProbeResult(id uint, status string, version *string, checkedAt time.Time) error {
	return s.db.Model(&model.ServiceInstance{}).Where("id = ?", id).Updates(map[string]any{
		"last_status":   status,
		"last_version":  version,
		"last_check_at": checkedAt,
	}).Error
}
