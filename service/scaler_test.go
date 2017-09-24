package service

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
)

type ScalerTestSuite struct {
	suite.Suite
	scaler     *ScalerService
	defaultMax uint64
	defaultMin uint64
	replicaMin uint64
	replicaMax uint64
	replicas   uint64
}

func TestScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ScalerTestSuite))
}

func (s *ScalerTestSuite) SetupSuite() {
	client, _ := client.NewEnvClient()
	defer client.Close()
	_, err := client.Info(context.Background())
	if err != nil {
		s.T().Skipf("Unable to connect to Docker Client")
	}
	_, err = client.SwarmInspect(context.Background())
	if err != nil {
		s.T().Skipf("Docker process is not a part of a swarm")
	}

	s.defaultMin = 1
	s.defaultMax = 10
	s.replicaMin = 2
	s.replicaMax = 4
	s.replicas = 3
	s.scaler = NewScalerService(
		client, "com.df.scaleMin", "com.df.scaleMax",
		s.defaultMin, s.defaultMax)
}

func (s *ScalerTestSuite) SetupTest() {
	cmd := fmt.Sprintf(`docker service create --name web_test \
		   -l com.df.scaleMin=%d \
		   -l com.df.scaleMax=%d \
		   --replicas %d \
		   -d \
		   alpine:3.6 \
		   sleep 10000000`, s.replicaMin, s.replicaMax, s.replicas)
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf("Unable to create service")
	}
}

func (s *ScalerTestSuite) TearDownTest() {
	cmd := `docker service rm web_test`
	exec.Command("/bin/sh", "-c", cmd).Output()
}

func (s *ScalerTestSuite) Test_GetReplicasServiceDoesNotExist() {
	_, err := s.scaler.GetReplicas("BADTEST")
	s.Error(err)
}

func (s *ScalerTestSuite) Test_GetReplicas() {
	replicas, err := s.scaler.GetReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(s.replicas, replicas)
}

func (s *ScalerTestSuite) Test_SetReplicas() {
	err := s.scaler.SetReplicas("web_test", 4)
	s.Require().NoError(err)
	replicas, err := s.scaler.GetReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(uint64(4), replicas)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicas() {
	min, max, err := s.scaler.GetMinMaxReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMaxLabel() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMax`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.GetMinMaxReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMin, min)
	s.Equal(s.defaultMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMinLabel() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.GetMinMaxReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(s.defaultMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoLabels() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin \
			--label-rm com.df.scaleMax`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.GetMinMaxReplicas("web_test")
	s.Require().NoError(err)
	s.Equal(s.defaultMin, min)
	s.Equal(s.defaultMax, max)
}
