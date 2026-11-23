package db

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"docker-updater/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB   *gorm.DB
	once sync.Once
)

// InitDB 初始化 SQLite 数据库
func InitDB() error {
	var err error
	once.Do(func() {
		pkgVar := utils.GetPkgVarDir()
		if err = os.MkdirAll(pkgVar, 0755); err != nil {
			return
		}
		dbPath := filepath.Join(pkgVar, "data.db")

		newLogger := logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Error,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		)

		DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: newLogger,
		})
		if err != nil {
			return
		}

		sqlDB, errDb := DB.DB()
		if errDb == nil {
			sqlDB.SetMaxOpenConns(1)
		}

		if keyErr := initCryptoKey(pkgVar); keyErr != nil {
			utils.LogWarning("无法初始化密钥: %s", keyErr.Error())
		}

		err = DB.AutoMigrate(
			&Setting{},
			&AvailableUpdate{},
			&DeferredUpdate{},
			&UpdateHistory{},
			&RegistryCredential{},
		)
		if err == nil {
			migrateLegacySettings()
		}
	})
	return err
}

// migrateLegacySettings 将历史配置键一次性迁移至新键并清理旧键
func migrateLegacySettings() {
	migrations := []struct {
		oldKey string
		newKey string
	}{
		{oldKey: "smtp_enabled", newKey: "notify_enabled"},
		{oldKey: "temp_mirrors", newKey: "dockerhub_mirrors"},
	}

	for _, m := range migrations {
		var oldSetting Setting
		if err := DB.First(&oldSetting, "key = ?", m.oldKey).Error; err != nil {
			continue
		}
		var newSetting Setting
		if err := DB.First(&newSetting, "key = ?", m.newKey).Error; err != nil {
			_ = SetSetting(m.newKey, oldSetting.Value)
			utils.LogInfo("配置迁移: 已将旧配置项 %s 迁移至 %s", m.oldKey, m.newKey)
		}
		_ = DB.Delete(&Setting{}, "key = ?", m.oldKey).Error
	}
}

// GetSetting 获取配置项
func GetSetting(key string, defaultVal string) string {
	var s Setting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		return defaultVal
	}
	return s.Value
}

// SetSetting 保存配置项
func SetSetting(key string, value string) error {
	s := Setting{Key: key, Value: value}
	return DB.Save(&s).Error
}
