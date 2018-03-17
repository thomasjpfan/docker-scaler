package service

import (
	"github.com/docker/docker/client"
)

var dockerAPIVersion = "v1.25"

// NewDockerClientFromEnv returns a `*client.Client` struct
func NewDockerClientFromEnv() (*client.Client, error) {
	host := "unix:///var/run/docker.sock"
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	return client.NewClient(host, dockerAPIVersion, nil, defaultHeaders)
}
