package integration

import (
	"bytes"
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
	dc                     *client.Client
	scaleURL               string
	targetService          string
	falseRescheduleService string
	alertURL               string
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
	dc, err := client.NewEnvClient()
	s.Require().NoError(err)
	s.dc = dc

	scalerIP := os.Getenv("SCALER_IP")
	targetService := os.Getenv("TARGET_SERVICE")
	alertAddress := os.Getenv("ALERTMANAGER_ADDRESS")
	falseRescheduleService := os.Getenv("FALSE_RESCHEDULE_SERVICE")
	s.Require().NotEmpty(scalerIP)
	s.Require().NotEmpty(targetService)
	s.Require().NotEmpty(falseRescheduleService)

	s.scaleURL = fmt.Sprintf("http://%s:8080/v1", scalerIP)
	s.targetService = targetService
	s.alertURL = alertAddress
	s.falseRescheduleService = falseRescheduleService
}

func (s *IntegrationTestSuite) Test_NoPOSTBody() {
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	req, _ := http.NewRequest("POST", url, nil)

	resp := s.responseForRequest(req, http.StatusBadRequest)
	message := "Unable to recognize POST body"
	s.Equal("NOK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "bad_request")
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	s.Equal(0, len(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_ScaleServiceNoServiceNameInBody() {
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := `{"groupLabels":{"scale":"up"}}`
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusBadRequest)
	message := "No service name in request body"
	s.Equal("NOK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "bad_request")
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	s.Equal(0, len(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_ScaleServiceNoScaleNameInBody() {
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := `{"groupLabels":{"service":"test_web"}}`
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusBadRequest)
	message := "No scale direction in request body"
	s.Equal("NOK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "bad_request")
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	s.Equal(0, len(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_ScaleServiceIncorrectScaleName() {
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"boo"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusBadRequest)
	message := "Incorrect scale direction in request body"
	s.Equal("NOK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "bad_request")
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	s.Equal(0, len(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_ServiceDoesNotExist() {
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := `{"groupLabels":{"service":"BAD", "scale":"up"}}`
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusInternalServerError)
	message := "docker inspect failed in ScalerService"
	s.Equal("NOK", resp.Status)
	s.True(strings.Contains(resp.Message, message))

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "error", "BAD")
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := "Scale service up: BAD"
	s.Equal(request, string(alert.Annotations["request"]))
	s.True(strings.Contains(string(alert.Annotations["summary"]), message))
}

func (s *IntegrationTestSuite) Test_ServiceScaledPassMax() {
	s.scaleService(s.targetService, 4)
	time.Sleep(1 * time.Second)

	// Scaled to 4 with com.df.scaleUpBy = 2 => bound by com.df.scaleMax
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"up"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusOK)
	message := fmt.Sprintf("Scaling %s from 4 to 5 replicas", s.targetService)
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service up: %s", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledToMax() {

	s.scaleService(s.targetService, 5)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the max of 5
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"up"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusOK)
	message := fmt.Sprintf("%s is already scaled to the maximum number of 5 replicas", s.targetService)
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service up: %s", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))

}

func (s *IntegrationTestSuite) Test_ServiceDescaledToMin() {

	s.scaleService(s.targetService, 2)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"down"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusOK)
	s.Require().Equal(2, s.getReplicas(s.targetService))

	message := fmt.Sprintf("%s is already descaled to the minimum number of 2 replicas", s.targetService)
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service down: %s", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledUp() {

	s.scaleService(s.targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 3
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"up"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusOK)
	s.Require().Equal(5, s.getReplicas(s.targetService))

	message := fmt.Sprintf("Scaling %s from 3 to 5 replicas", s.targetService)
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service up: %s", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_ServiceScaledDown() {

	s.scaleService(s.targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale-service", s.scaleURL)
	jsonStr := fmt.Sprintf(`{"groupLabels":{"service":"%s", "scale":"down"}}`, s.targetService)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	resp := s.responseForRequest(req, http.StatusOK)
	s.Require().Equal(2, s.getReplicas(s.targetService))

	message := fmt.Sprintf("Scaling %s from 3 to 2 replicas", s.targetService)
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	// Check alert
	alerts, err := service.FetchAlerts(s.alertURL, "scale_service", "success", s.targetService)
	s.Require().NoError(err)
	s.Require().Equal(1, len(alerts))

	alert := alerts[0]
	request := fmt.Sprintf("Scale service down: %s", s.targetService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))
}

func (s *IntegrationTestSuite) Test_RescheduleAll() {

	url := fmt.Sprintf("%s/reschedule-services", s.scaleURL)
	req, _ := http.NewRequest("POST", url, nil)

	resp := s.responseForRequest(req, http.StatusOK)

	message := "Rescheduled all services"
	s.Equal("OK", resp.Status)
	s.Equal(message, resp.Message)

	alerts, err := service.FetchAlerts(s.alertURL, "reschedule_services", "success", "reschedule")
	s.Require().NoError(err)
	s.Require().Len(alerts, 1)

	alert := alerts[0]
	request := "Rescheduling all labeled services"
	s.Equal(request, string(alert.Annotations["request"]))

	serviceInfo, _, err := s.dc.ServiceInspectWithRaw(context.Background(), s.targetService)
	s.Require().NoError(err)
	spec := &serviceInfo.Spec
	envs := spec.TaskTemplate.ContainerSpec.Env

	var target1 string
	for _, env := range envs {
		envSplit := strings.Split(env, "=")
		s.Require().Len(envSplit, 2)
		if envSplit[0] == "RESCHEDULE_DATE" {
			target1 = envSplit[1]
			break
		}
	}

	s.NotEqual("", target1)

	serviceInfo, _, err = s.dc.ServiceInspectWithRaw(context.Background(), s.falseRescheduleService)
	s.Require().NoError(err)
	spec = &serviceInfo.Spec
	envs = spec.TaskTemplate.ContainerSpec.Env

	var target2 string
	for _, env := range envs {
		envSplit := strings.Split(env, "=")
		s.Require().Len(envSplit, 2)
		if envSplit[0] == "RESCHEDULE_DATE" {
			target2 = envSplit[1]
			break
		}
	}

	s.Equal("", target2)
}

func (s *IntegrationTestSuite) Test_RescheduleOneFalseOne() {

	url := fmt.Sprintf("%s/reschedule-service?service=%s", s.scaleURL, s.falseRescheduleService)
	req, _ := http.NewRequest("POST", url, nil)

	resp := s.responseForRequest(req, http.StatusInternalServerError)

	message := fmt.Sprintf("%s is not labeled with com.df.reschedule=true (com.df.reschedule=false)", s.falseRescheduleService)
	s.Equal("NOK", resp.Status)
	s.Equal(message, resp.Message)

	alerts, err := service.FetchAlerts(s.alertURL, "reschedule_service", "error", "reschedule")
	s.Require().NoError(err)
	s.Require().Len(alerts, 1)

	alert := alerts[0]
	request := fmt.Sprintf("Rescheduling service: %s", s.falseRescheduleService)
	s.Equal(request, string(alert.Annotations["request"]))
	s.Equal(message, string(alert.Annotations["summary"]))

	serviceInfo, _, err := s.dc.ServiceInspectWithRaw(context.Background(), s.falseRescheduleService)
	s.Require().NoError(err)
	spec := &serviceInfo.Spec
	envs := spec.TaskTemplate.ContainerSpec.Env

	var target string
	for _, env := range envs {
		envSplit := strings.Split(env, "=")
		s.Require().Len(envSplit, 2)
		if envSplit[0] == "RESCHEDULE_DATE" {
			target = envSplit[1]
			break
		}
	}

	s.Equal("", target)

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

func (s *IntegrationTestSuite) responseForRequest(request *http.Request, code int) *server.Response {

	require := s.Require()
	resp, err := http.DefaultClient.Do(request)
	require.NoError(err)

	s.Equal(code, resp.StatusCode)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	require.NotNil(m)

	return &m
}
