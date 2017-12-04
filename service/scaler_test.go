package service

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
)

type ScalerTestSuite struct {
	suite.Suite
	scaler             *scalerService
	ctx                context.Context
	defaultMax         uint64
	defaultMin         uint64
	replicaMin         uint64
	replicaMax         uint64
	replicas           uint64
	scaleUpBy          uint64
	scaleDownBy        uint64
	defaultScaleDownBy uint64
	defaultScaleUpBy   uint64
}

func TestScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ScalerTestSuite))
}

func (s *ScalerTestSuite) SetupSuite() {
	client, err := client.NewEnvClient()
	if err != nil {
		s.T().Skipf("Unable to connect to Docker Client")
	}
	defer client.Close()
	_, err = client.Info(context.Background())
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
	s.replicaMax = 6
	s.replicas = 4
	s.scaleDownBy = 1
	s.scaleUpBy = 2
	s.defaultScaleDownBy = 1
	s.defaultScaleUpBy = 1
	s.ctx = context.Background()
	s.scaler = &scalerService{
		c:                  client,
		minLabel:           "com.df.scaleMin",
		maxLabel:           "com.df.scaleMax",
		scaleDownByLabel:   "com.df.scaleDownBy",
		scaleUpByLabel:     "com.df.scaleUpBy",
		defaultMin:         s.defaultMin,
		defaultMax:         s.defaultMax,
		defaultScaleDownBy: s.defaultScaleDownBy,
		defaultScaleUpBy:   s.defaultScaleUpBy}
}

func (s *ScalerTestSuite) SetupTest() {
	cmd := fmt.Sprintf(`docker service create --name web_test \
		   -l com.df.scaleMin=%d \
		   -l com.df.scaleMax=%d \
		   -l com.df.scaleDownBy=%d \
		   -l com.df.scaleUpBy=%d \
		   --replicas %d \
		   -d \
		   alpine:3.6 \
		   sleep 10000000`, s.replicaMin, s.replicaMax,
		s.scaleDownBy, s.scaleUpBy, s.replicas)
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf("Unable to create service: %s", err.Error())
	}
}

func (s *ScalerTestSuite) TearDownTest() {
	cmd := `docker service rm web_test`
	exec.Command("/bin/sh", "-c", cmd).Output()
	time.Sleep(time.Millisecond * 500)
}

func (s *ScalerTestSuite) Test_GetReplicasServiceDoesNotExist() {
	_, err := s.scaler.getReplicas(s.ctx, "BADTEST")
	s.Error(err)
}

func (s *ScalerTestSuite) Test_GetReplicas() {
	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.replicas, replicas)
}

func (s *ScalerTestSuite) Test_SetReplicas() {
	err := s.scaler.setReplicas(s.ctx, "web_test", 4)
	s.Require().NoError(err)
	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(uint64(4), replicas)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicas() {
	min, max, err := s.scaler.getMinMaxReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMaxLabel() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMax -d`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.getMinMaxReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMin, min)
	s.Equal(s.defaultMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMinLabel() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin -d`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.getMinMaxReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.defaultMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoLabels() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin \
			--label-rm com.df.scaleMax -d`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.getMinMaxReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.defaultMin, min)
	s.Equal(s.defaultMax, max)
}

func (s *ScalerTestSuite) Test_GetDownUpScaleDeltas() {
	scaleDownBy, scaleUpBy, err := s.scaler.getDownUpScaleDeltas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.scaleDownBy, scaleDownBy)
	s.Equal(s.scaleUpBy, scaleUpBy)
}

func (s *ScalerTestSuite) Test_GetDownUpScaleDeltasNoLabels() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleDownBy \
			--label-rm com.df.scaleUpBy -d`
	exec.Command("/bin/sh", "-c", cmd).Output()
	min, max, err := s.scaler.getDownUpScaleDeltas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.defaultScaleDownBy, min)
	s.Equal(s.defaultScaleUpBy, max)
}

func (s *ScalerTestSuite) Test_AlreadyAtMax() {
	expMsg := fmt.Sprintf("web_test is already scaled to the maximum number of %d replicas", s.replicaMax)

	err := s.scaler.setReplicas(s.ctx, "web_test", s.replicaMax)
	msg, isBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(isBounded)
	s.Equal(expMsg, msg)

	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMax, replicas)
}

func (s *ScalerTestSuite) Test_AlreadyAtMin() {
	expMsg := fmt.Sprintf("web_test is already descaled to the minimum number of %d replicas", s.replicaMin)

	err := s.scaler.setReplicas(s.ctx, "web_test", s.replicaMin)
	msg, isBounded, err := s.scaler.ScaleDown(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(isBounded)
	s.Equal(expMsg, msg)

	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(s.replicaMin, replicas)
}

func (s *ScalerTestSuite) Test_ScaleUpBy_PassMax() {
	oldReplicas := s.replicaMax - 1
	newReplicas := s.replicaMax
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (max: %d)", oldReplicas, newReplicas, s.replicaMax)

	err := s.scaler.setReplicas(s.ctx, "web_test", oldReplicas)
	msg, isBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(isBounded)
	s.Equal(expMsg, msg)

	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(newReplicas, replicas)
}

func (s *ScalerTestSuite) Test_ScaleUp() {
	newReplicas := s.replicas + s.scaleUpBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (max: %d)", s.replicas, newReplicas, s.replicaMax)

	msg, isBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(isBounded)
	s.Equal(expMsg, msg)

	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(newReplicas, replicas)
}

func (s *ScalerTestSuite) Test_ScaleDown() {

	newReplicas := s.replicas - s.scaleDownBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d)", s.replicas, newReplicas, s.replicaMin)

	msg, isBounded, err := s.scaler.ScaleDown(s.ctx, "web_test")
	s.Require().NoError(err)
	s.False(isBounded)
	s.Equal(expMsg, msg)

	replicas, err := s.scaler.getReplicas(s.ctx, "web_test")
	s.Require().NoError(err)
	s.Equal(newReplicas, replicas)
}
