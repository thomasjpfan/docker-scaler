package service

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/suite"
)

type DockerClientTestSuite struct {
	suite.Suite
	client  DockerClient
	ctx     context.Context
	service swarm.Service
}

func TestDockerClientUnitTestSuite(t *testing.T) {
	suite.Run(t, new(DockerClientTestSuite))
}

func (s *DockerClientTestSuite) SetupSuite() {
	client, err := NewDockerClientFromEnv()
	if err != nil {
		s.T().Skipf("Unable to create Docker Client")
	}
	s.client = client
	s.ctx = context.Background()

}

func (s *DockerClientTestSuite) TearDownSuite() {
	s.client.Close()
}

func (s *DockerClientTestSuite) SetupTest() {
	cmd := `docker service create --name web_test \
		   -l com.df.notify=true \
		   -e HELLO=WORLD \
		   --replicas 2 -d alpine:3.6 \
		   sleep 10000000`

	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf("Unable to create service: %s", err.Error())
	}

	tickerC := time.NewTicker(time.Millisecond * 500).C
	timerC := time.NewTimer(time.Second * 10).C

L:
	for {
		select {
		case <-tickerC:
			service, _, err := s.client.dc.ServiceInspectWithRaw(
				s.ctx, "web_test", types.ServiceInspectOptions{})
			if err == nil {
				s.service = service
				break L
			}
		case <-timerC:
			break L
		}
	}

	if err != nil {
		s.T().Skipf("Unable to create service: %s", err.Error())
	}
}
func (s *DockerClientTestSuite) TearDownTest() {
	cmd := `docker service rm web_test`
	exec.Command("/bin/sh", "-c", cmd).Output()
}

func (s *DockerClientTestSuite) Test_ServiceInspect_WithName() {
	ss, err := s.client.ServiceInspect(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal("web_test", ss.Spec.Name)

	s.Require().NotNil(ss.Spec.Mode.Replicated)
	s.Require().NotNil(ss.Spec.Mode.Replicated.Replicas)
	s.Equal(uint64(2), *ss.Spec.Mode.Replicated.Replicas)
}

func (s *DockerClientTestSuite) Test_ServiceInspect_WithID() {
	ss, err := s.client.ServiceInspect(s.ctx, s.service.ID)
	s.Require().NoError(err)
	s.Equal(s.service.ID, ss.ID)

	s.Require().NotNil(ss.Spec.Mode.Replicated)
	s.Require().NotNil(ss.Spec.Mode.Replicated.Replicas)
	s.Equal(uint64(2), *ss.Spec.Mode.Replicated.Replicas)
}

func (s *DockerClientTestSuite) Test_ServiceInspect_ServiceDoesNotExist_ReturnsError() {
	_, err := s.client.ServiceInspect(s.ctx, "wow")
	s.Require().Error(err)
}

func (s *DockerClientTestSuite) Test_ServiceUpdateReplicas() {
	newReplicas := uint64(3)
	spec := s.service.Spec
	spec.Mode.Replicated.Replicas = &newReplicas
	err := s.client.ServiceUpdate(
		s.ctx, s.service.ID, s.service.Version, spec)
	s.Require().NoError(err)

	newService, err := s.client.ServiceInspect(s.ctx, s.service.ID)
	s.Require().NoError(err)

	s.Require().NotNil(newService.Spec.Mode.Replicated)
	s.Require().NotNil(newService.Spec.Mode.Replicated.Replicas)
	s.Equal(uint64(3), *newService.Spec.Mode.Replicated.Replicas)
}

func (s *DockerClientTestSuite) Test_ServiceUpdateEnv() {
	spec := s.service.Spec
	envs := spec.TaskTemplate.ContainerSpec.Env
	s.Require().Len(envs, 1)
	s.Require().Contains(envs, "HELLO=WORLD")

	envs = append(envs, "DOG=CAT")
	spec.TaskTemplate.ContainerSpec.Env = envs

	err := s.client.ServiceUpdate(
		s.ctx, s.service.ID, s.service.Version, spec)
	s.Require().NoError(err)

	newService, err := s.client.ServiceInspect(s.ctx, s.service.ID)
	s.Require().NoError(err)
	newSpec := newService.Spec
	newEnvs := newSpec.TaskTemplate.ContainerSpec.Env
	s.Require().Len(newEnvs, 2)
	s.Require().Contains(newEnvs, "DOG=CAT")
}

func (s *DockerClientTestSuite) Test_Info_Manager() {
	cnt, err := s.client.NodeReadyCnt(s.ctx, true)
	s.Require().NoError(err)

	s.Equal(1, cnt)
}

func (s *DockerClientTestSuite) Test_Info_Worker() {
	cnt, err := s.client.NodeReadyCnt(s.ctx, false)
	s.Require().NoError(err)

	s.Equal(0, cnt)
}

func (s *DockerClientTestSuite) Test_ServiceList_TrueLabel() {
	f := filters.NewArgs()
	f.Add("label", "com.df.notify=true")
	sList, err := s.client.ServiceList(
		s.ctx, types.ServiceListOptions{Filters: f})
	s.Require().NoError(err)

	s.Len(sList, 1)
}

func (s *DockerClientTestSuite) Test_ServiceList_FalseLabel() {
	f := filters.NewArgs()
	f.Add("label", "com.df.notify=false")
	sList, err := s.client.ServiceList(
		s.ctx, types.ServiceListOptions{Filters: f})
	s.Require().NoError(err)

	s.Len(sList, 0)
}
