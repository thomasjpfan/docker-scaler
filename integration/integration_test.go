package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
	"github.com/thomasjpfan/docker-scaler/server"
)

type IntegrationTestSuite struct {
	suite.Suite
	dc            *client.Client
	url           string
	targetService string
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
	s.Require().NotEmpty(scalerIP)
	s.Require().NotEmpty(targetService)

	s.url = fmt.Sprintf("http://%s:8080", scalerIP)
	s.targetService = targetService
}

func (s *IntegrationTestSuite) Test_NonIntegerDeltaQuery() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=%s&delta=what", s.url, s.targetService)
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
	s.Equal("NOK", m.Status)
	s.Equal("Incorrect delta query: what", m.Message)
}

func (s *IntegrationTestSuite) Test_ServiceDoesNotExist() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=BAD&delta=1", s.url)
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
	s.Equal("NOK", m.Status)
}

func (s *IntegrationTestSuite) Test_DeltaResultsInNegativeReplicas() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=%s&delta=-100", s.url, s.targetService)
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
	s.Equal("NOK", m.Status)
	s.Equal(fmt.Sprintf("Delta -100 results in a negative number of replicas for service: %s", s.targetService), m.Message)
}

func (s *IntegrationTestSuite) Test_ServiceScaledToMax() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 4)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the max of 4
	url := fmt.Sprintf("%s/scale?service=%s&delta=1", s.url, targetService)
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
	s.Equal("NOK", m.Status)
	s.Equal(fmt.Sprintf("%s is already scaled to the maximum number of 4 replicas", s.targetService), m.Message)
}

func (s *IntegrationTestSuite) Test_ServiceDescaledToMin() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 2)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale?service=%s&delta=-1", s.url, targetService)
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
	s.Equal("NOK", m.Status)
	s.Equal(fmt.Sprintf("%s is already descaled to the minimum number of 2 replicas", s.targetService), m.Message)
}

func (s *IntegrationTestSuite) Test_ServiceScaledUp() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 3
	url := fmt.Sprintf("%s/scale?service=%s&delta=1", s.url, targetService)
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
	s.Equal("OK", m.Status)
	s.Equal(fmt.Sprintf("Scaling %s to 4 replicas", s.targetService), m.Message)
}

func (s *IntegrationTestSuite) Test_ServiceScaledDown() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)
	time.Sleep(1 * time.Second)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale?service=%s&delta=-1", s.url, targetService)
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
