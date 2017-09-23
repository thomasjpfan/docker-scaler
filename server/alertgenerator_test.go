package server

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AlertGeneratorTestSuite struct {
	suite.Suite
}

func TestAlertGeneratorUnitTestSuite(t *testing.T) {
	suite.Run(t, new(AlertGeneratorTestSuite))
}

func (s *AlertGeneratorTestSuite) Test_GenerateAlert() {
	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	message := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"

	alert := GenerateAlert(alertname, serviceName, status, message, request)
	s.Require().NotNil(alert)
	s.Equal(alertname, string(alert.Labels["alertname"]))
	s.Equal(serviceName, string(alert.Labels["service"]))
	s.Equal(status, string(alert.Labels["status"]))
	s.Equal(message, string(alert.Annotations["message"]))
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal("", alert.GeneratorURL)
}
