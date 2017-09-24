package service

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AlertTestSuite struct {
	suite.Suite
}

func TestAlertUnitTestSuite(t *testing.T) {
	suite.Run(t, new(AlertTestSuite))
}

func (s *AlertTestSuite) Test_generateAlert() {
	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	message := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"

	alert := generateAlert(alertname, serviceName, request, status, message)
	s.Require().NotNil(alert)
	s.Equal(alertname, string(alert.Labels["alertname"]))
	s.Equal(serviceName, string(alert.Labels["service"]))
	s.Equal(status, string(alert.Labels["status"]))
	s.Equal(message, string(alert.Annotations["message"]))
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal("", alert.GeneratorURL)
}
