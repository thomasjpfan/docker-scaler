package service

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type SilentAlertTestSuite struct {
	suite.Suite
}

func TestSilentAlertTestSuite(t *testing.T) {
	suite.Run(t, new(SilentAlertTestSuite))
}

func (s *SilentAlertTestSuite) Test_SilentAlert_NoError() {
	sa := NewSilentAlertService()
	err := sa.Send("alertName", "serviceName", "request", "status", "message")
	s.NoError(err)
}
