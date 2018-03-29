package service

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
)

type ScalerTestSuite struct {
	suite.Suite
	scaler             *scalerService
	ctx                context.Context
	client             *client.Client
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
	client, err := NewDockerClientFromEnv()
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
	s.client = client
	s.ctx = context.Background()
	s.scaler = NewScalerService(
		client,
		"com.df.scaleMin",
		"com.df.scaleMax",
		"com.df.scaleDownBy",
		"com.df.scaleUpBy",
		s.defaultMin,
		s.defaultMax,
		s.defaultScaleDownBy,
		s.defaultScaleUpBy).(*scalerService)
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

	tickerC := time.NewTicker(time.Millisecond * 500).C
	timerC := time.NewTimer(time.Second * 10).C

L:
	for {
		select {
		case <-tickerC:
			_, _, err = s.client.ServiceInspectWithRaw(
				s.ctx, "web_test", types.ServiceInspectOptions{})
			if err == nil {
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

func (s *ScalerTestSuite) TearDownTest() {
	cmd := `docker service rm web_test`
	exec.Command("/bin/sh", "-c", cmd).Output()

	tickerC := time.NewTicker(time.Millisecond * 500).C
	timerC := time.NewTimer(time.Second * 10).C

	var err error
L:
	for {
		select {
		case <-tickerC:
			_, _, err = s.client.ServiceInspectWithRaw(
				s.ctx, "web_test", types.ServiceInspectOptions{})
			if err != nil {
				break L
			}
		case <-timerC:
			break L
		}
	}
}

func (s *ScalerTestSuite) Test_isGlobal() {
	_, err := s.scaler.isGlobal(swarm.Service{})
	s.Error(err)
}

func (s *ScalerTestSuite) Test_GetReplicas() {
	ts := s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(s.replicas, replicas)
}

func (s *ScalerTestSuite) Test_SetReplicas() {

	ts := s.getTestService()
	err := s.scaler.setReplicas(s.ctx, ts, 4)
	s.Require().NoError(err)

	ts = s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(uint64(4), replicas)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicas() {

	ts := s.getTestService()
	min, max := s.scaler.getMinMaxReplicas(ts)
	s.Equal(s.replicaMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMaxLabel() {

	cmd := `docker service update web_test \
			--label-rm com.df.scaleMax -d`
	exec.Command("/bin/sh", "-c", cmd).Output()

	ts := s.getTestService()
	min, max := s.scaler.getMinMaxReplicas(ts)
	s.Equal(s.replicaMin, min)
	s.Equal(s.defaultMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoMinLabel() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin -d`

	exec.Command("/bin/sh", "-c", cmd).Output()
	ts := s.getTestService()
	min, max := s.scaler.getMinMaxReplicas(ts)
	s.Equal(s.defaultMin, min)
	s.Equal(s.replicaMax, max)
}

func (s *ScalerTestSuite) Test_GetMinMaxReplicasNoLabels() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleMin \
			--label-rm com.df.scaleMax -d`
	exec.Command("/bin/sh", "-c", cmd).Output()

	ts := s.getTestService()
	min, max := s.scaler.getMinMaxReplicas(ts)
	s.Equal(s.defaultMin, min)
	s.Equal(s.defaultMax, max)
}

func (s *ScalerTestSuite) Test_GetDownUpScaleDeltas() {

	ts := s.getTestService()
	scaleDownBy, scaleUpBy := s.scaler.getScaleUpDownDeltas(ts)
	s.Equal(s.scaleDownBy, scaleDownBy)
	s.Equal(s.scaleUpBy, scaleUpBy)
}

func (s *ScalerTestSuite) Test_GetDownUpScaleDeltasNoLabels() {
	cmd := `docker service update web_test \
			--label-rm com.df.scaleDownBy \
			--label-rm com.df.scaleUpBy -d`
	exec.Command("/bin/sh", "-c", cmd).Output()

	ts := s.getTestService()
	min, max := s.scaler.getScaleUpDownDeltas(ts)
	s.Equal(s.defaultScaleDownBy, min)
	s.Equal(s.defaultScaleUpBy, max)
}

func (s *ScalerTestSuite) Test_AlreadyAtMax() {
	expMsg := fmt.Sprintf("web_test is already scaled to the maximum number of %d replicas", s.replicaMax)

	ts := s.getTestService()
	err := s.scaler.setReplicas(s.ctx, ts, s.replicaMax)
	msg, alreadyBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(alreadyBounded)
	s.Equal(expMsg, msg)

	ts = s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(s.replicaMax, replicas)
}

func (s *ScalerTestSuite) Test_AlreadyAtMin() {
	expMsg := fmt.Sprintf("web_test is already descaled to the minimum number of %d replicas", s.replicaMin)

	ts := s.getTestService()
	err := s.scaler.setReplicas(s.ctx, ts, s.replicaMin)
	msg, alreadyBounded, err := s.scaler.ScaleDown(s.ctx, "web_test")
	s.Require().NoError(err)
	s.True(alreadyBounded)
	s.Equal(expMsg, msg)

	ts = s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(s.replicaMin, replicas)
}

func (s *ScalerTestSuite) Test_ScaleUpBy_PassMax() {
	oldReplicas := s.replicaMax - 1
	newReplicas := s.replicaMax
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (max: %d)", oldReplicas, newReplicas, s.replicaMax)

	ts := s.getTestService()
	err := s.scaler.setReplicas(s.ctx, ts, oldReplicas)
	time.Sleep(time.Second)
	msg, alreadyBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	ts = s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(newReplicas, replicas)
}

func (s *ScalerTestSuite) Test_ScaleUp() {
	newReplicas := s.replicas + s.scaleUpBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (max: %d)", s.replicas, newReplicas, s.replicaMax)

	msg, alreadyBounded, err := s.scaler.ScaleUp(s.ctx, "web_test")
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	ts := s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Equal(newReplicas, replicas)
}

func (s *ScalerTestSuite) Test_ScaleUp_ServiceDoesNotExist() {
	_, _, err := s.scaler.ScaleUp(s.ctx, "NOT_EXIST")
	s.Error(err)
}

func (s *ScalerTestSuite) Test_ScaleDown() {

	newReplicas := s.replicas - s.scaleDownBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d)", s.replicas, newReplicas, s.replicaMin)

	msg, alreadyBounded, err := s.scaler.ScaleDown(s.ctx, "web_test")
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	ts := s.getTestService()
	replicas := s.scaler.getReplicas(ts)
	s.Require().NoError(err)
	s.Equal(newReplicas, replicas)
}

func (s *ScalerTestSuite) Test_ScaleDown_ServiceDoesNotExist() {
	_, _, err := s.scaler.ScaleDown(s.ctx, "NOT_EXIST")
	s.Error(err)
}

func (s *ScalerTestSuite) Test_GlobalService_ScaleUpAndScaleDown_ReturnsError() {
	cmd := fmt.Sprintf(`docker service create --name web_global \
		   -l com.df.scaleMin=%d \
		   -l com.df.scaleMax=%d \
		   -l com.df.scaleDownBy=%d \
		   -l com.df.scaleUpBy=%d \
		   --mode global \
		   -d \
		   alpine:3.6 \
		   sleep 10000000`, s.replicaMin, s.replicaMax,
		s.scaleDownBy, s.scaleUpBy)
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf("Unable to create service: %s", err.Error())
	}

	tickerC := time.NewTicker(time.Millisecond * 500).C
	timerC := time.NewTimer(time.Second * 10).C
	var globalService swarm.Service

L:
	for {
		select {
		case <-tickerC:
			globalService, _, err = s.client.ServiceInspectWithRaw(
				s.ctx, "web_global", types.ServiceInspectOptions{})
			if err == nil {
				break L
			}
		case <-timerC:
			break L
		}
	}

	if err != nil {
		s.T().Skipf("Unable to create service: %s", err.Error())
	}

	isGlobal, err := s.scaler.isGlobal(globalService)
	s.NoError(err)
	s.True(isGlobal)

	_, _, err = s.scaler.ScaleUp(s.ctx, "web_global")
	s.Error(err)
	s.Contains(err.Error(), "web_global is a global service (can not be scaled)")

	_, _, err = s.scaler.ScaleDown(s.ctx, "web_global")
	s.Error(err)
	s.Contains(err.Error(), "web_global is a global service (can not be scaled)")

	cmd = `docker service rm web_global`
	exec.Command("/bin/sh", "-c", cmd).Output()
}

func (s *ScalerTestSuite) getTestService() swarm.Service {
	service, _, err := s.client.ServiceInspectWithRaw(
		s.ctx, "web_test", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	return service
}
