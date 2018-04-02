package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

type ScalerTestSuite struct {
	suite.Suite
	scaler      *scalerService
	ctx         context.Context
	clientMock  *DockerClientMock
	replicaMin  uint64
	replicaMax  uint64
	replicas    uint64
	scaleUpBy   uint64
	scaleDownBy uint64
	opts        ResolveDeltaOptions
}

func TestScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ScalerTestSuite))
}

func (s *ScalerTestSuite) SetupSuite() {
	s.replicas = 4

	s.replicaMin = 2
	s.replicaMax = 6
	s.scaleDownBy = 1
	s.scaleUpBy = 2

	s.opts = ResolveDeltaOptions{
		MinLabel:           "com.df.scaleMin",
		MaxLabel:           "com.df.scaleMax",
		ScaleDownByLabel:   "com.df.scaleDownBy",
		ScaleUpByLabel:     "com.df.scaleUpBy",
		DefaultMin:         1,
		DefaultMax:         10,
		DefaultScaleDownBy: 3,
		DefaultScaleUpBy:   3,
	}
	s.ctx = context.Background()
}

func (s *ScalerTestSuite) SetupTest() {

	s.clientMock = new(DockerClientMock)
	s.scaler = NewScalerService(s.clientMock, s.opts).(*scalerService)
}

func (s *ScalerTestSuite) Test_Scale_UnrecognizedService() {
	ss := swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "wow",
			},
		},
	}
	s.clientMock.On(
		"ServiceInspect", s.ctx, "wow").
		Return(ss, nil)
	_, _, err := s.scaler.Scale(s.ctx, "wow", 0, ScaleUpDirection)
	s.Require().Error(err)

	s.Contains(err.Error(), "Unable to recognize service model for: wow")
}

func (s *ScalerTestSuite) Test_Scale_UnrecognizedReplicas() {
	ss := swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "wow",
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: nil,
				},
			},
		},
	}
	s.clientMock.On(
		"ServiceInspect", s.ctx, "wow").
		Return(ss, nil)
	_, _, err := s.scaler.Scale(s.ctx, "wow", 0, ScaleUpDirection)
	s.Require().Error(err)

	s.Contains(err.Error(), "wow does not have a replicas value")
}

func (s *ScalerTestSuite) Test_ScaleUp_ServiceDoesNotExist() {
	expErr := errors.New("Does not exist")
	s.clientMock.On(
		"ServiceInspect", s.ctx, "NOT_EXIST").
		Return(swarm.Service{}, expErr)
	_, _, err := s.scaler.Scale(s.ctx, "NOT_EXIST", 0, ScaleUpDirection)
	s.Require().Error(err)

	s.Contains(err.Error(), "docker inspect failed in ScalerService")
}

func (s *ScalerTestSuite) Test_ScaleDown_ServiceDoesNotExist() {
	expErr := errors.New("Does not exist")
	s.clientMock.On(
		"ServiceInspect", s.ctx, "NOT_EXIST").
		Return(swarm.Service{}, expErr)
	_, _, err := s.scaler.Scale(s.ctx, "NOT_EXIST", 0, ScaleDownDirection)
	s.Require().Error(err)

	s.Contains(err.Error(), "docker inspect failed in ScalerService")
}

func (s *ScalerTestSuite) Test_ScaleUp_UpdateError() {
	oldReplicas := s.replicaMax - 1
	newReplicas := s.replicaMax

	prevts, newts := s.getTestService(), s.getTestService()
	prevts.Spec.Mode.Replicated.Replicas = &oldReplicas
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	expErr := errors.New("Unable to update")

	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(prevts, nil).
		On("ServiceUpdate", s.ctx, "web_testID", prevts.Version,
			newts.Spec).
		Return(expErr)

	_, _, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().Error(err)

	s.Equal("Unable to update", err.Error())
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUp_AlreadyAtMax() {
	expMsg := fmt.Sprintf("web_test is already scaled to the maximum number of %d replicas", s.replicaMax)

	ts := s.getTestService()
	ts.Spec.Mode.Replicated.Replicas = &s.replicaMax
	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().NoError(err)
	s.True(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDown_AlreadyAtMin() {
	expMsg := fmt.Sprintf("web_test is already descaled to the minimum number of %d replicas", s.replicaMin)

	ts := s.getTestService()
	ts.Spec.Mode.Replicated.Replicas = &s.replicaMin
	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)
	s.Require().NoError(err)
	s.True(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUpBy_PassMax() {
	oldReplicas := s.replicaMax - 1
	newReplicas := s.replicaMax
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", oldReplicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	prevts.Spec.Mode.Replicated.Replicas = &oldReplicas
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUpBy_PassDefaultMax() {
	oldReplicas := s.opts.DefaultMax - 1
	newReplicas := s.opts.DefaultMax
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", oldReplicas, newReplicas, s.replicaMin, s.opts.DefaultMax)

	prevts, newts := s.getTestService(), s.getTestService()
	prevts.Spec.Mode.Replicated.Replicas = &oldReplicas
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	delete(prevts.Spec.Labels, "com.df.scaleMax")
	delete(newts.Spec.Labels, "com.df.scaleMax")
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUp() {
	newReplicas := s.replicas + s.scaleUpBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUp_CustomBy() {
	newReplicas := s.replicas + 1
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 1, ScaleUpDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleUp_DefaultBy() {

	newReplicas := s.replicas + 3
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.opts.DefaultMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	delete(prevts.Spec.Labels, "com.df.scaleUpBy")
	delete(newts.Spec.Labels, "com.df.scaleUpBy")
	delete(prevts.Spec.Labels, "com.df.scaleMax")
	delete(newts.Spec.Labels, "com.df.scaleMax")
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDownBy_PassMin() {
	oldReplicas := s.replicaMin + 1
	newReplicas := s.replicaMin
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", oldReplicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	prevts.Spec.Mode.Replicated.Replicas = &oldReplicas
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDownBy_PassDefaultMin() {
	oldReplicas := s.opts.DefaultMin + 1
	newReplicas := s.opts.DefaultMin
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", oldReplicas, newReplicas, s.opts.DefaultMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	prevts.Spec.Mode.Replicated.Replicas = &oldReplicas
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	delete(prevts.Spec.Labels, "com.df.scaleMin")
	delete(newts.Spec.Labels, "com.df.scaleMin")
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDown() {

	newReplicas := s.replicas - s.scaleDownBy
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDown_CustomBy() {

	newReplicas := s.replicas - 2
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 2, ScaleDownDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_ScaleDown_DefaultBy() {

	newReplicas := s.replicaMin
	expMsg := fmt.Sprintf("Scaling web_test from %d to %d replicas (min: %d, max: %d)", s.replicas, newReplicas, s.replicaMin, s.replicaMax)

	prevts, newts := s.getTestService(), s.getTestService()
	newts.Spec.Mode.Replicated.Replicas = &newReplicas
	delete(prevts.Spec.Labels, "com.df.scaleDownBy")
	delete(newts.Spec.Labels, "com.df.scaleDownBy")
	s.setClientMock(prevts, newts)

	msg, alreadyBounded, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)
	s.Require().NoError(err)
	s.False(alreadyBounded)
	s.Equal(expMsg, msg)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_GlobalService_ScaleUp_ReturnsError() {
	ts := s.getTestService()
	ts.Spec.Mode.Replicated = nil
	ts.Spec.Mode.Global = &swarm.GlobalService{}
	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	_, _, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleUpDirection)

	s.Require().Error(err)
	s.Contains(err.Error(), "web_test is a global service (can not be scaled)")
	s.clientMock.AssertExpectations(s.T())
}

func (s *ScalerTestSuite) Test_GlobalService_ScaleDown_ReturnsError() {
	ts := s.getTestService()
	ts.Spec.Mode.Replicated = nil
	ts.Spec.Mode.Global = &swarm.GlobalService{}
	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	_, _, err := s.scaler.Scale(s.ctx, "web_test", 0, ScaleDownDirection)

	s.Require().Error(err)
	s.Contains(err.Error(), "web_test is a global service (can not be scaled)")
	s.clientMock.AssertExpectations(s.T())

}
func (s *ScalerTestSuite) getTestService() swarm.Service {
	labels := map[string]string{
		"com.df.scaleMin":    "2",
		"com.df.scaleMax":    "6",
		"com.df.scaleDownBy": "1",
		"com.df.scaleUpBy":   "2",
	}
	return swarm.Service{
		ID: "web_testID",
		Meta: swarm.Meta{
			Version: swarm.Version{
				Index: uint64(1),
			}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   "web_test",
				Labels: labels,
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &s.replicas,
				},
			},
		},
	}
}

func (s *ScalerTestSuite) setClientMock(prevService, nextService swarm.Service) {
	s.clientMock.On(
		"ServiceInspect", s.ctx, "web_test").
		Return(prevService, nil).
		On("ServiceUpdate", s.ctx, "web_testID", prevService.Version,
			nextService.Spec).
		Return(nil)
}
