package model

import "time"

// ServiceInstance 被管理的服务实例：通用内核的核心域模型。
// 全部字段数据化配置，内核不硬编码任何特定服务的逻辑；
// 新增一个被管理服务 = 注册配置 +（必要时）一个 adapter，不改内核代码。
type ServiceInstance struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `gorm:"size:30;uniqueIndex" json:"name"`
	BaseURL string `gorm:"size:255" json:"baseUrl"`

	AuthType        string `gorm:"size:10;default:none" json:"authType"` // none | bearer
	TokenCiphertext string `gorm:"size:255" json:"-"`                    // SecretBox 密文，永不下发
	HasToken        bool   `gorm:"-" json:"hasToken"`                    // 是否已配置凭据（service 层填充）
	HealthPath      string `gorm:"size:100;default:/api/health" json:"healthPath"`
	Enabled         bool   `gorm:"default:true" json:"enabled"`

	LastCheckAt *time.Time `json:"lastCheckAt,omitempty"`
	LastStatus  string     `gorm:"size:10;default:unknown" json:"lastStatus"` // ok | fail | unknown
	LastVersion *string    `gorm:"size:20" json:"lastVersion,omitempty"`
	Note        string     `gorm:"size:255" json:"note,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"-"`
}

// FillHasToken 按凭据密文有无填充 hasToken（service 层在响应前调用）。
func (i *ServiceInstance) FillHasToken() {
	i.HasToken = i.TokenCiphertext != ""
}

// FillHasTokenAll 批量填充（列表响应用）。
func FillHasTokenAll(in []ServiceInstance) []ServiceInstance {
	for idx := range in {
		in[idx].FillHasToken()
	}
	return in
}
