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

// InitDB 初始化 SQLite 数据库。
func InitDB() error {
	var err error
	once.Do(func() {
		pkgVar := os.Getenv("TRIM_PKGVAR")
		if pkgVar == "" {
			pkgVar = "./data"
		}
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
			&RollbackMetadata{},
			&RegistryCredential{},
		)
	})
	return err
}

// GetSetting 获取配置。
func GetSetting(key string, defaultVal string) string {
	var s Setting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		return defaultVal
	}
	return s.Value
}

// SetSetting 保存配置。
func SetSetting(key string, value string) error {
	s := Setting{Key: key, Value: value}
	return DB.Save(&s).Error
}
