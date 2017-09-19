package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
	"github.com/thomasjpfan/docker-scaler/server"
)

type IntegrationTestSuite struct {
	suite.Suite
	dc            *client.Client
	endpoint      string
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

	s.endpoint = fmt.Sprintf("http://%s:8080", scalerIP)
	s.targetService = targetService
}

func (s *IntegrationTestSuite) SetupTest() {

}

func (s *IntegrationTestSuite) TearDownTest() {

}

func (s *IntegrationTestSuite) Test_NonIntegerDeltaQuery() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=%s&delta=what", s.endpoint, s.targetService)
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
	s.Equal(m.Status, "NOK")
	s.Equal(m.Message, "Incorrect delta query: what")
}

func (s *IntegrationTestSuite) Test_ServiceDoesNotExist() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=BAD&delta=1", s.endpoint)
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
	s.Equal(m.Status, "NOK")
}

func (s *IntegrationTestSuite) Test_DeltaResultsInNegativeReplicas() {
	require := s.Require()
	url := fmt.Sprintf("%s/scale?service=%s&delta=-100", s.endpoint, s.targetService)
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
	s.Equal(m.Status, "NOK")
	s.Equal(m.Message, fmt.Sprintf("Delta -100 results in a negative number of replicas for service: %s", s.targetService))
}

func (s *IntegrationTestSuite) Test_ServiceScaledToMax() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 4)

	// Now service is scaled to the max of 4
	url := fmt.Sprintf("%s/scale?service=%s&delta=1", s.endpoint, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusPreconditionFailed, resp.StatusCode)
	require.Equal(s.getReplicas(targetService), 4)

	require.Equal(http.StatusBadRequest, resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	s.Equal(m.Status, "NOK")
	s.Equal(m.Message, fmt.Sprintf("%s is already scaled to the maximum number of 4 replicas", s.targetService))
}

func (s *IntegrationTestSuite) Test_ServiceDescaledToMin() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 2)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale?service=%s&delta=-1", s.endpoint, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusPreconditionFailed, resp.StatusCode)
	require.Equal(s.getReplicas(targetService), 2)

	require.Equal(http.StatusBadRequest, resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	s.Equal(m.Status, "NOK")
	s.Equal(m.Message, fmt.Sprintf("%s is already scaled to the minimum number of 4 replicas", s.targetService))
}

func (s *IntegrationTestSuite) Test_ServiceScaledUp() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)

	// Now service is scaled to the min of 3
	url := fmt.Sprintf("%s/scale?service=%s&delta=1", s.endpoint, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(s.getReplicas(targetService), 4)

	require.Equal(http.StatusBadRequest, resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	s.Equal(m.Status, "OK")
	s.Equal(m.Message, fmt.Sprintf("Scaling %s to 4 replicas", s.targetService))
}

func (s *IntegrationTestSuite) Test_ServiceScaledDown() {

	require := s.Require()
	targetService := s.targetService
	s.scaleService(targetService, 3)

	// Now service is scaled to the min of 2
	url := fmt.Sprintf("%s/scale?service=%s&delta=-1", s.endpoint, targetService)
	req, _ := http.NewRequest("POST", url, nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.NotNil(resp)

	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(s.getReplicas(targetService), 2)

	require.Equal(http.StatusBadRequest, resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(err)

	var m server.Response
	err = json.Unmarshal(body, &m)
	require.NoError(err)
	s.Equal(m.Status, "OK")
	s.Equal(m.Message, fmt.Sprintf("Scaling %s to 2 replicas", s.targetService))
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
