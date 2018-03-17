package service

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	s.alertService = &alertService{
		url:          s.url,
		alertTimeout: time.Second * 15,
	}
	s.client = client
}

func (s *AlertTestSuite) TearDownSuite() {
	s.client.Close()
}

func (s *AlertTestSuite) SetupTest() {
	cmd := `docker run --name am9093 -p 9093:9093 \
			-d prom/alertmanager:v0.13.0`
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		s.T().Skipf(fmt.Sprintf("Unable to create alertmanager: %s", err.Error()))
		return
	}

	// Wait for am to come online
	for i := 1; i <= 60; i++ {
		info, _ := s.client.ContainerInspect(context.Background(), "am9093")
		if info.State.Running {
			time.Sleep(1 * time.Second)
			return
		}
		time.Sleep(1 * time.Second)
	}
	s.T().Skipf("Unable to create alertmanager")
}

func (s *AlertTestSuite) TearDownTest() {
	cmd := "docker container rm -f am9093"
	exec.Command("/bin/sh", "-c", cmd).Output()
}

func (s *AlertTestSuite) Test_SendAlert() {
	require := s.Require()
	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	summary := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"

	s.alertService.Send(alertname, serviceName, request, status, summary)
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

func Test_generateAlert(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	serviceName := "web"
	alertname := "service_scaler"
	status := "success"
	summary := "Scaled web from 3 to 4 replicas"
	request := "Scale web with delta=1"
	startsAt := time.Now().UTC()
	timeout := time.Second
	endsAt := startsAt.Add(timeout)

	alert := generateAlert(alertname, serviceName, request, status, summary, startsAt, timeout)
	require.NotNil(alert)
	assert.Equal(alertname, string(alert.Labels["alertname"]))
	assert.Equal(serviceName, string(alert.Labels["service"]))
	assert.Equal(status, string(alert.Labels["status"]))
	assert.Equal(summary, string(alert.Annotations["summary"]))
	assert.Equal(request, string(alert.Annotations["request"]))
	assert.Equal(startsAt, alert.StartsAt)
	assert.Equal(endsAt, alert.EndsAt)
	assert.Equal("", alert.GeneratorURL)
}
