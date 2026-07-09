package service

import (
	"context"
	"docker-updater/db"
	"docker-updater/dockerclient"
)

// DBCredentialsProvider 本地凭证提供者。
type DBCredentialsProvider struct{}

// GetCredential 获取数据库凭证。
func (DBCredentialsProvider) GetCredential(ctx context.Context, registry string) (username, password string, ok bool) {
	var cred db.RegistryCredential
	if err := db.DB.First(&cred, "registry = ?", registry).Error; err == nil {
		return cred.Username, cred.Password, true
	}
	return "", "", false
}

func init() {
	dockerclient.GlobalCredentialsProvider = DBCredentialsProvider{}
}
