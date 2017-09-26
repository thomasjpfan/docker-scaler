package service

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type StubbedNodeScalerTestSuite struct {
	suite.Suite
	sns *StubbedNodeScaler
}

func TestStubbedNodeScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(StubbedNodeScalerTestSuite))
}

func (s *StubbedNodeScalerTestSuite) SetupSuite() {
	s.sns = new(StubbedNodeScaler)
}

func (s *StubbedNodeScalerTestSuite) Test_SetNodes() {
	err := s.sns.SetNodes(uint64(1))
	s.Equal("No backend configured for node scaling", err.Error())
}

func (s *StubbedNodeScalerTestSuite) Test_GetNodes() {
	_, err := s.sns.GetNodes()
	s.Equal("No backend configured for node scaling", err.Error())
}
