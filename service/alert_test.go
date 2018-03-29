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

type AlertTestSuite struct {
	suite.Suite
	url          string
	alertService AlertServicer
	client       *client.Client
}

func TestAlertUnitTestSuite(t *testing.T) {
	suite.Run(t, new(AlertTestSuite))
}

func (s *AlertTestSuite) SetupSuite() {
	client, _ := NewDockerClientFromEnv()
	_, err := client.Info(context.Background())
	if err != nil {
		s.T().Skipf("Unable to connect to Docker Client")
	}
	s.url = "http://localhost:9093"
	s.alertService = NewAlertService(s.url, time.Second*15)
	s.client = client
}

func (s *AlertTestSuite) TearDownSuite() {
	s.client.Close()
}

func (s *AlertTestSuite) Test_SendAlert() {

	defer func() {
		cmd := "docker container stop am9093"
		exec.Command("/bin/sh", "-c", cmd).Output()
	}()
	cmd := `docker run --name am9093 --rm -p 9093:9093 \
			-d prom/alertmanager:v0.14.0`
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf(fmt.Sprintf("Unable to create alertmanager: %s", err.Error()))
		return
	}

	running := false
	// Wait for am to come online
	for i := 1; i <= 60; i++ {
		info, _ := s.client.ContainerInspect(context.Background(), "am9093")
		if info.State.Running {
			running = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !running {
		s.T().Skipf(fmt.Sprintf("Alertmanager not created"))
		return
	}

	require := s.Require()
	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	summary := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"

	err = s.alertService.Send(alertname, serviceName, request, status, summary)
	require.NoError(err)
	time.Sleep(1 * time.Second)

	alerts, err := FetchAlerts(s.url, alertname, status, serviceName)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	s.Equal(alertname, string(alert.Labels["alertname"]))
	s.Equal(serviceName, string(alert.Labels["service"]))
	s.Equal(status, string(alert.Labels["status"]))
	s.Equal(summary, string(alert.Annotations["summary"]))
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal("", alert.GeneratorURL)
}

func (s *AlertTestSuite) Test_generateAlert() {

	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	summary := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"
	startsAt := time.Now().UTC()
	timeout := time.Second
	endsAt := startsAt.Add(timeout)

	alert := generateAlert(alertname, serviceName, request, status, summary, startsAt, timeout)
	s.Require().NotNil(alert)
	s.Equal(alertname, string(alert.Labels["alertname"]))
	s.Equal(serviceName, string(alert.Labels["service"]))
	s.Equal(status, string(alert.Labels["status"]))
	s.Equal(summary, string(alert.Annotations["summary"]))
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(startsAt, alert.StartsAt)
	s.Equal(endsAt, alert.EndsAt)
	s.Equal("", alert.GeneratorURL)
}
