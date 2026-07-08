package db

// Setting 存储系统配置项的键值对
type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// AvailableUpdate 存储检测到有新版本的本地容器信息
type AvailableUpdate struct {
	ContainerName  string `gorm:"primaryKey"`
	Image          string
	LocalDigest    string
	RemoteDigest   string
	CheckedAt      string
	ComposeProject string
}

// DeferredUpdate 存储用户手动设置的延迟更新限制
type DeferredUpdate struct {
	ContainerName string `gorm:"primaryKey"`
	Until         string // YYYY-MM-DD
}

// UpdateHistory 存储最近的更新与回退执行历史
type UpdateHistory struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	ContainerName string
	Image         string
	UpdatedAt     string
	Status        string // success 或 error:[错误描述]
}

// RollbackMetadata 存储旧版本备份容器的元数据
type RollbackMetadata struct {
	ContainerName string `gorm:"primaryKey"`
	BackedUpAt    string
	ExpiresAt     string
	RestartPolicy string // JSON 序列化字符串，保存原始容器的重启策略
}

// RegistryCredential 存储私有镜像仓库的认证凭据
type RegistryCredential struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Registry  string `gorm:"uniqueIndex" json:"registry"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	UpdatedAt string `json:"updated_at"`
}

