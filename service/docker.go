package service

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

var dockerAPIVersion = "v1.25"

// DockerClient wraps `*client.Client` in docker
type DockerClient struct {
	dc *client.Client
}

// NewDockerClientFromEnv returns a `*client.Client` struct
func NewDockerClientFromEnv() (DockerClient, error) {
	host := "unix:///var/run/docker.sock"
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	c, err := client.NewClient(host, dockerAPIVersion, nil, defaultHeaders)
	if err != nil {
		return DockerClient{}, err
	}
	return DockerClient{c}, nil
}

// ServiceInspect wraps `dc.ServiceInspect`
func (c DockerClient) ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error) {
	service, _, err := c.dc.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	return service, err
}

// ServiceUpdate wraps `dc.ServiceUpdate`
func (c DockerClient) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec) error {
	_, err := c.dc.ServiceUpdate(
		ctx, serviceID, version, service,
		types.ServiceUpdateOptions{
			RegistryAuthFrom: types.RegistryAuthFromSpec,
		})
	return err
}

// NodeReadyCnt wraps `dc.NodeList`
func (c DockerClient) NodeReadyCnt(ctx context.Context, manager bool) (int, error) {
	var typeStr string
	if manager {
		typeStr = "manager"
	} else {
		typeStr = "worker"
	}
	f := filters.NewArgs()
	f.Add("role", typeStr)
	nodes, err := c.dc.NodeList(ctx, types.NodeListOptions{Filters: f})
	if err != nil {
		return 0, err
	}

	cnt := 0
	for _, n := range nodes {
		if n.Status.State == swarm.NodeStateReady {
			cnt++
		}
	}

	return cnt, nil
}

// ServiceList wraps `dc.ServiceList`
func (c DockerClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	return c.dc.ServiceList(ctx, options)
}

// Close wraps `dc.Close`
func (c DockerClient) Close() {
	c.dc.Close()
}
