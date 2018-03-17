package service

import (
	"context"
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
