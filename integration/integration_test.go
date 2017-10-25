package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

type IntegrationTestSuite struct {
	suite.Suite
	dc            *client.Client
	scaleURL      string
	targetService string
	alertURL      string
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
	dc, err := client.NewEnvClient()
	s.Require().Nil(err)
	s.dc = dc

	scalerIP := os.Getenv("SCALER_IP")
	targetService := os.Getenv("TARGET_SERVICE")
	alertAddress := os.Getenv("ALERTMANAGER_ADDRESS")
	s.Require().NotEmpty(scalerIP)
	s.Require().NotEmpty(targetService)

	s.scaleURL = fmt.Sprintf("http://%s:8080/v1", scalerIP)
	s.targetService = targetService
	s.alertURL = fmt.Sprintf("http://%s:9093", alertAddress)
}

func (s *IntegrationTestSuite) Test_NonIntegerDeltaQuery() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=what", s.scaleURL, s.targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)

	s.Equal(http.StatusBadRequest, resp.StatusCode)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := "Incorrect delta query: what"
	s.Equal("NOK", m.Status)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: what", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceDoesNotExist() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale-service?name=BAD&delta=1", s.scaleURL)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusInternalServerError, resp.StatusCode)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := "docker inspect failed in ScalerService"
	s.Equal("NOK", m.Status)
	s.True(strings.Contains(m.Message, message))

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "BAD")
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := "Scale service: BAD, delta: 1"
	s.Equal(request, string(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_DeltaResultsInNegativeReplicas() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=-100", s.scaleURL, s.targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusBadRequest, resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := fmt.Sprintf("Delta -100 results in a negative number of replicas for service: %s", s.targetService)
	s.Equal("NOK", m.Status)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: -100", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledToMax() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 4)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the max of 4
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=1", s.scaleURL, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusPreconditionFailed, resp.StatusCode)
	require.Equal(s.getReplicas(targetService), 4)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := fmt.Sprintf("%s is already scaled to the maximum number of 4 replicas", s.targetService)
	s.Equal("NOK", m.Status)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: 1", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))

}

func (s *IntegrationTestSuite) Test_ServiceDescaledToMin() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 2)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=-1", s.scaleURL, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusPreconditionFailed, resp.StatusCode)
	require.Equal(2, s.getReplicas(targetService))

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := fmt.Sprintf("%s is already descaled to the minimum number of 2 replicas", s.targetService)
	s.Equal("NOK", m.Status)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: -1", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledUp() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 3
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=1", s.scaleURL, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(4, s.getReplicas(targetService))

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	message := fmt.Sprintf("Scaling %s to 4 replicas", s.targetService)
	s.Equal("OK", m.Status)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: 1", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledDown() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale-service?name=%s&delta=-1", s.scaleURL, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(2, s.getReplicas(targetService))

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	s.Equal("OK", m.Status)

	message := fmt.Sprintf("Scaling %s to 2 replicas", targetService)
	s.Equal(message, m.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	require.NoError(err)
	require.Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service: %s, delta: -1", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) scaleService(serviceName string, count uint64) {
	require := s.Require()

	service, _, err := s.dc.ServiceInspectWithRaw(context.Background(), serviceName)
	require.NoError(err)

	service.Spec.Mode.Replicated.Replicas = &count
	updateOpts := types.ServiceUpdateOptions{}
	updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec

	_, updateErr := s.dc.ServiceUpdate(
		context.Background(), service.ID, service.Version, service.Spec, updateOpts)
	require.NoError(updateErr)
}

func (s *IntegrationTestSuite) getReplicas(serviceName string) int {

	require := s.Require()
	service, _, err := s.dc.ServiceInspectWithRaw(context.Background(), serviceName)
	require.NoError(err)

	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return int(currentReplicas)
}
