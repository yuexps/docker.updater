package dockerclient

import (
	"github.com/docker/docker/client"
)

// NewLocalClient 创建本地 Docker 客户端，并自动协商 API 版本
func NewLocalClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}
