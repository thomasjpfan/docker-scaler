package service

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/mock"
)

type DockerClientMock struct {
	mock.Mock
}

func (m *DockerClientMock) ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error) {
	called := m.Called(ctx, serviceID)
	return called.Get(0).(swarm.Service), called.Error(1)
}

func (m *DockerClientMock) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec) error {
	called := m.Called(ctx, serviceID, version, service)
	return called.Error(0)
}

func (m *DockerClientMock) Info(ctx context.Context) (types.Info, error) {
	called := m.Called(ctx)
	return called.Get(0).(types.Info), called.Error(1)
}

func (m *DockerClientMock) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	called := m.Called(ctx, options)
	return called.Get(0).([]swarm.Service), called.Error(1)
}
