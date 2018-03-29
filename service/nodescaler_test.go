package service

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type NodeScalerTestSuite struct {
	suite.Suite
}

func TestNodeScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(NodeScalerTestSuite))
}

func (s *NodeScalerTestSuite) Test_SilentNodeScaler_ErrorsOnScale() {
	silentScaler, err := NewNodeScaler("silent")
	s.Require().NoError(err)

	ctx := context.Background()
	_, _, err = silentScaler.ScaleWorkerByDelta(ctx, 1)
	s.Error(err)

	_, _, err = silentScaler.ScaleManagerByDelta(ctx, -1)
	s.Error(err)
}

func (s *NodeScalerTestSuite) Test_NewNodeScalerAWS_DidNotSetRequiredEnvs() {
	_, err := NewNodeScaler("aws")
	s.Error(err)
}

func (s *NodeScalerTestSuite) Test_NewNodeScalerAWS_AWS_ENV_FILE_DoesNotExist() {
	defer func() {
		os.Unsetenv("AWS_ENV_FILE")
	}()
	os.Setenv("AWS_ENV_FILE", "notafile")
	_, err := NewNodeScaler("aws")
	s.Error(err)
	s.Contains(err.Error(), "Unable to load notafile")
}

func (s *NodeScalerTestSuite) Test_NewNodeScalerAWS_ASGDefined() {
	defer func() {
		os.Unsetenv("AWS_MANAGER_ASG")
		os.Unsetenv("AWS_WORKER_ASG")
	}()
	os.Setenv("AWS_MANAGER_ASG", "awsmanager")
	os.Setenv("AWS_WORKER_ASG", "awsworker")
	nodeScaler, err := NewNodeScaler("aws")
	s.Require().NoError(err)
	s.NotNil(nodeScaler)
}
