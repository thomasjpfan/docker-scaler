package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/thomasjpfan/docker-scaler/service"
)

type ScalerServicerMock struct {
	mock.Mock
}

func (m *ScalerServicerMock) GetReplicas(ctx context.Context, serviceName string) (uint64, error) {
	args := m.Called(serviceName)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *ScalerServicerMock) SetReplicas(ctx context.Context, serviceName string, count uint64) error {
	args := m.Called(serviceName, count)
	return args.Error(0)
}

func (m *ScalerServicerMock) GetMinMaxReplicas(ctx context.Context, serviceName string) (uint64, uint64, error) {
	args := m.Called(serviceName)
	return args.Get(0).(uint64), args.Get(1).(uint64), args.Error(2)
}

type AlertServicerMock struct {
	mock.Mock
}

func (am *AlertServicerMock) Send(alertName string, serviceName string, status string, message string, request string) error {
	args := am.Called(alertName, serviceName, status, message, request)
	return args.Error(0)
}

type NodeScalerMock struct {
	mock.Mock
}

func (nsm *NodeScalerMock) ScaleManagerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	args := nsm.Called(delta)
	return args.Get(0).(uint64), args.Get(1).(uint64), args.Error(2)
}

func (nsm *NodeScalerMock) ScaleWorkerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	args := nsm.Called(delta)
	return args.Get(0).(uint64), args.Get(1).(uint64), args.Error(2)
}

type NodeScalerCreaterMock struct {
	mock.Mock
}

func (nscm *NodeScalerCreaterMock) New(nodeBackend string) (service.NodeScaler, error) {
	args := nscm.Called(nodeBackend)
	return args.Get(0).(*NodeScalerMock), args.Error(1)
}

type ServerTestSuite struct {
	suite.Suite
	m    *ScalerServicerMock
	am   *AlertServicerMock
	nsm  *NodeScalerMock
	nscm *NodeScalerCreaterMock
	s    *Server
	r    *mux.Router
	l    *log.Logger
	b    *bytes.Buffer
}

func TestServerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (suite *ServerTestSuite) SetupTest() {
	suite.m = new(ScalerServicerMock)
	suite.am = new(AlertServicerMock)
	suite.nsm = new(NodeScalerMock)
	suite.nscm = new(NodeScalerCreaterMock)

	suite.b = new(bytes.Buffer)
	suite.l = log.New(suite.b, "", 0)
	suite.s = NewServer(suite.m, suite.am,
		suite.nscm, suite.l)
	suite.r = suite.s.MakeRouter()
}

func (suite *ServerTestSuite) Test_NonIntegerDeltaQuery() {

	require := suite.Require()
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		url := fmt.Sprintf("/v1/scale-service?name=hello&delta=%v", deltaStr)
		requestMessage := fmt.Sprintf("Scale service: hello, delta: %s", deltaStr)
		errorMessage := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		suite.am.On("Send", "scale_service", "hello",
			requestMessage, "error", errorMessage).Return(nil)

		req, _ := http.NewRequest("POST", url, nil)
		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		require.Equal(http.StatusBadRequest, rec.Code)

		suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		suite.RequireLogs(suite.b.String(), requestMessage, errorMessage)
		suite.am.AssertExpectations(suite.T())
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_DeltaResultsNegativeReplica() {

	require := suite.Require()
	requestMessage := "Scale service: web, delta: -10"
	errorMessage := "Delta -10 results in a negative number of replicas for service: web"
	url := "/v1/scale-service?name=web&delta=-10"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", errorMessage).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), requestMessage, errorMessage)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleService_DoesNotExist() {
	require := suite.Require()
	expErr := fmt.Errorf("No such service: web")
	requestMessage := "Scale service: web, delta: 1"
	url := "/v1/scale-service?name=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(0), uint64(0), expErr)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusInternalServerError, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, expErr.Error())
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_ScaledToMax() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: 1"
	expErr := fmt.Errorf("web is already scaled to the maximum number of 4 replicas")
	url := "/v1/scale-service?name=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, expErr.Error())
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_DescaledToMin() {

	require := suite.Require()
	requestMessage := "Scale service: web, delta: -1"
	expErr := fmt.Errorf("web is already descaled to the minimum number of 2 replicas")
	url := "/v1/scale-service?name=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(2), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, expErr.Error())
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerUp() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: 1"
	message := "Scaling web to 4 replicas"
	url := "/v1/scale-service?name=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(4)).Return(nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerDown() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: -1"
	message := "Scaling web to 2 replicas"
	url := "/v1/scale-service?name=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(2)).Return(nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_NonIntegerDeltaQuery() {
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		url := fmt.Sprintf("/v1/scale-nodes?backend=mock&delta=%v&type=worker", deltaStr)
		requestMessage := fmt.Sprintf("Scale node on: mock, delta: %s, type: worker", deltaStr)
		errorMessage := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		suite.am.On("Send", "scale_node", "mock",
			requestMessage, "error", errorMessage).Return(nil)
		suite.nscm.On("New", "mock").Return(suite.nsm, nil)

		req, _ := http.NewRequest("POST", url, nil)
		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		suite.Equal(http.StatusBadRequest, rec.Code)

		suite.RequireLogs(suite.b.String(), requestMessage, errorMessage)

		suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		suite.am.AssertExpectations(suite.T())
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_ScaleNode_BackEndDoesNotExist() {
	url := "/v1/scale-nodes?backend=BADSERVICE&delta=1&type=worker"
	requestMessage := "Scale node on: BADSERVICE, delta: 1, type: worker"
	expErr := fmt.Errorf("BADSERVICE does not exist")

	suite.nscm.On("New", "BADSERVICE").Return(suite.nsm, expErr)
	suite.am.On("Send", "scale_node", "BADSERVICE", requestMessage, "error", expErr.Error()).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusPreconditionFailed, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, expErr.Error())
	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.nscm.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleByDeltaError() {

	url := "/v1/scale-nodes?backend=mock&delta=1&type=worker"
	requestMessage := "Scale node on: mock, delta: 1, type: worker"
	expErr := fmt.Errorf("Unable to scale node")

	suite.nscm.On("New", "mock").Return(suite.nsm, nil)
	suite.am.On("Send", "scale_node", "mock", requestMessage, "error", expErr.Error()).Return(nil)
	suite.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(0), uint64(0), expErr)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusInternalServerError, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, expErr.Error())
	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.nscm.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
	suite.nsm.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleWithBadType() {
	url := "/v1/scale-nodes?backend=mock&delta=1&type=BAD"
	requestMessage := "Scale node on: mock, delta: 1, type: BAD"
	message := "Incorrect type: BAD, type can only be worker or manager"
	suite.am.On("Send", "scale_node", "mock", requestMessage, "error", message).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusPreconditionFailed, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", message)
}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleWorkerByDelta() {
	url := "/v1/scale-nodes?backend=mock&delta=1&type=worker"
	requestMessage := "Scale node on: mock, delta: 1, type: worker"
	message := "Changed the number of worker nodes on mock from 3 to 4"

	suite.nscm.On("New", "mock").Return(suite.nsm, nil)
	suite.am.On("Send", "scale_node", "mock", requestMessage, "success", message).Return(nil)
	suite.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(3), uint64(4), nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusOK, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.nscm.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
	suite.nsm.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleManagerByDelta() {
	url := "/v1/scale-nodes?backend=mock&delta=-1&type=manager"
	requestMessage := "Scale node on: mock, delta: -1, type: manager"
	message := "Changed the number of manager nodes on mock from 3 to 2"

	suite.nscm.On("New", "mock").Return(suite.nsm, nil)
	suite.am.On("Send", "scale_node", "mock", requestMessage, "success", message).Return(nil)
	suite.nsm.On("ScaleManagerByDelta", -1).Return(uint64(3), uint64(2), nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusOK, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.nscm.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
	suite.nsm.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) RequireLogs(logMessage string, expectedLogs ...string) {
	logMessages := strings.Split(logMessage, "\n")
	suite.Require().True(len(logMessages) >= len(expectedLogs))
	for i, m := range expectedLogs {
		suite.Equal(m, logMessages[i])
	}
}

func (suite *ServerTestSuite) RequireResponse(data []byte, status string, message string) {
	var m Response
	err := json.Unmarshal(data, &m)
	suite.Require().NoError(err)

	suite.Equal(status, m.Status)
	suite.Equal(message, m.Message)
}
