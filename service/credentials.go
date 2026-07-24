package service

import (
	"context"
	"strings"

	"docker-updater/db"
	"docker-updater/dockerclient"
)

// DBCredentialsProvider 本地凭证提供者
type DBCredentialsProvider struct{}

// GetCredential 获取数据库凭证
func (DBCredentialsProvider) GetCredential(ctx context.Context, registry string) (username, password string, ok bool) {
	var list []db.RegistryCredential
	if err := db.DB.Find(&list).Error; err == nil {
		target := normalizeRegistry(registry)
		for _, cred := range list {
			if normalizeRegistry(cred.Registry) == target {
				return cred.Username, cred.Password, true
			}
		}
	}
	return "", "", false
}

func normalizeRegistry(reg string) string {
	r := strings.TrimSpace(reg)
	r = strings.TrimPrefix(r, "https://")
	r = strings.TrimPrefix(r, "http://")
	r = strings.TrimSuffix(r, "/")
	if r == "docker.io" || r == "index.docker.io" || r == "registry.docker.io" {
		return "registry-1.docker.io"
	}
	return r
}

func init() {
	dockerclient.GlobalCredentialsProvider = DBCredentialsProvider{}
}
