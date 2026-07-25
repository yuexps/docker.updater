package db

import (
	"gorm.io/gorm"
)

// Setting 系统配置表
type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// AvailableUpdate 容器版本检测结果表
type AvailableUpdate struct {
	ContainerName  string `gorm:"primaryKey"`
	Image          string
	LocalDigest    string
	RemoteDigest   string
	LocalVersion   string
	RemoteVersion  string
	CheckedAt      string
	ComposeProject string
	CheckMethod    string
	HasUpdate      bool // 是否存在可用更新
}

// DeferredUpdate 延迟更新约束表
type DeferredUpdate struct {
	ContainerName string `gorm:"primaryKey"`
	Until         string
}

// UpdateHistory 更新历史记录表
type UpdateHistory struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	ContainerName string
	Image         string
	UpdatedAt     string
	Status        string
}

// RegistryCredential 镜像仓库凭证表
type RegistryCredential struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Registry  string `gorm:"uniqueIndex" json:"registry"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	UpdatedAt string `json:"updated_at"`
}

// BeforeSave 敏感配置加密
func (s *Setting) BeforeSave(tx *gorm.DB) (err error) {
	if s.Key == "smtp_password" && s.Value != "" && s.Value != "******" {
		enc, err := encrypt(s.Value)
		if err != nil {
			return err
		}
		s.Value = enc
	}
	return nil
}

// AfterFind 敏感配置解密
func (s *Setting) AfterFind(tx *gorm.DB) (err error) {
	if s.Key == "smtp_password" && s.Value != "" {
		dec, err := decrypt(s.Value)
		if err != nil {
			return err
		}
		s.Value = dec
	}
	return nil
}

// BeforeSave 仓库凭据加密
func (rc *RegistryCredential) BeforeSave(tx *gorm.DB) (err error) {
	if rc.Password != "" && rc.Password != "******" {
		enc, err := encrypt(rc.Password)
		if err != nil {
			return err
		}
		rc.Password = enc
	}
	return nil
}

// AfterFind 仓库凭据解密
func (rc *RegistryCredential) AfterFind(tx *gorm.DB) (err error) {
	if rc.Password != "" {
		dec, err := decrypt(rc.Password)
		if err != nil {
			return err
		}
		rc.Password = dec
	}
	return nil
}
