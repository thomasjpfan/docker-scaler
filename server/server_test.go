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

func (m *ScalerServicerMock) GetDownUpScaleDeltas(ctx context.Context, serviceName string) (uint64, uint64, error) {
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

func (nsm *NodeScalerMock) String() string {
	return "mock"
}

type ServerTestSuite struct {
	suite.Suite
	m   *ScalerServicerMock
	am  *AlertServicerMock
	nsm *NodeScalerMock
	s   *Server
	r   *mux.Router
	l   *log.Logger
	b   *bytes.Buffer
}

func TestServerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (suite *ServerTestSuite) SetupTest() {
	suite.m = new(ScalerServicerMock)
	suite.am = new(AlertServicerMock)
	suite.nsm = new(NodeScalerMock)

	suite.b = new(bytes.Buffer)
	suite.l = log.New(suite.b, "", 0)
	suite.s = NewServer(suite.m, suite.am,
		suite.nsm, suite.l)
	suite.r = suite.s.MakeRouter()
}

func (suite *ServerTestSuite) Test_ScaleService_NoBody() {
	require := suite.Require()
	errorMessage := "No POST body"
	logMessage := fmt.Sprintf("scale-service error: %s", errorMessage)
	url := "/v1/scale-service"
	suite.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_NoServiceNameInBody() {
	require := suite.Require()
	errorMessage := "No service name in request body"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"scale": "up"}}`
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)
	suite.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_NoScaleDirectionInBody() {
	require := suite.Require()
	errorMessage := "No scale direction in request body"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"service": "web"}}`
	suite.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleService_IncorrectScaleName() {
	require := suite.Require()
	errorMessage := "Incorrect scale direction in request body"
	jsonStr := `{"groupLabels":{"service": "web", "scale": "boo"}}`
	suite.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)
	url := "/v1/scale-service"
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleService_DoesNotExist() {
	require := suite.Require()
	expErr := fmt.Errorf("No such service: web")
	logMessage := fmt.Sprintf("scale-service error: %s", expErr)
	requestMessage := "Scale service up: web"
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "up")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(0), uint64(0), expErr)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusInternalServerError, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, logMessage)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_ScaledToMax() {
	require := suite.Require()
	requestMessage := "Scale service up: web"
	expErr := fmt.Errorf("web is already scaled to the maximum number of 4 replicas")
	logMessage := fmt.Sprintf("scale-service error: %s", expErr)
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "up")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(1), uint64(1), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, logMessage)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_DescaledToMin() {

	require := suite.Require()
	requestMessage := "Scale service down: web"
	expErr := fmt.Errorf("web is already descaled to the minimum number of 2 replicas")
	url := "/v1/scale-service"
	logMessage := fmt.Sprintf("scale-service error: %s", expErr)
	body := NewScaleRequestBody("web", "down")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(2), nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(1), uint64(1), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.RequireLogs(suite.b.String(), requestMessage, logMessage)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerUp() {
	require := suite.Require()
	requestMessage := "Scale service up: web"
	message := "Scaling web from 3 to 4 replicas"
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "up")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(4)).Return(nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(1), uint64(1), nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerUpPassMax() {
	require := suite.Require()
	requestMessage := "Scale service up: web"
	message := "Scaling web from 3 to 5 replicas"
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "up")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(5), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(5)).Return(nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(1), uint64(3), nil)
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
	requestMessage := "Scale service down: web"
	message := "Scaling web from 3 to 2 replicas"
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "down")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(2)).Return(nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(1), uint64(1), nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerDownPassMin() {
	require := suite.Require()
	requestMessage := "Scale service down: web"
	message := "Scaling web from 3 to 1 replicas"
	url := "/v1/scale-service"
	body := NewScaleRequestBody("web", "down")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(1)).Return(nil)
	suite.m.On("GetDownUpScaleDeltas", "web").Return(uint64(3), uint64(1), nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_NoPOSTBody() {
	require := suite.Require()
	errorMessage := "No POST body"
	logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
	url := "/v1/scale-nodes?by=1&type=worker"
	suite.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_NoScaleDirectionInBody() {
	require := suite.Require()
	errorMessage := "No scale direction in request body"
	url := "/v1/scale-nodes?by=1&type=worker"
	jsonStr := `{"groupLabels":{}}`
	logMessage := fmt.Sprintf("scale-nodes error: %s, body: %s", errorMessage, jsonStr)
	suite.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	suite.RequireLogs(suite.b.String(), logMessage)
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_InvalidScaleValue() {
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		errorMessage := "Incorrect scale direction in request body"
		url := "/v1/scale-nodes?by=1&type=worker"
		jsonStr := fmt.Sprintf(`{"groupLabels":{"scale":"%s"}}`, deltaStr)
		logMessage := fmt.Sprintf("scale-nodes error: %s, body: %s", errorMessage, jsonStr)
		suite.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
		req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.RequireLogs(suite.b.String(), logMessage)
		suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		suite.am.AssertExpectations(suite.T())
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_ScaleNode_NonIntegerByQuery() {

	tt := []string{"what", "2114what", "24y4", "he"}

	for _, byStr := range tt {

		errorMessage := fmt.Sprintf("Non integer by query parameter: %s", byStr)
		url := fmt.Sprintf("/v1/scale-nodes?by=%s&type=worker", byStr)
		jsonStr := `{"groupLabels":{"scale":"up"}}`
		logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
		suite.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
		req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		suite.Equal(http.StatusBadRequest, rec.Code)

		suite.RequireLogs(suite.b.String(), logMessage)
		suite.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		suite.am.AssertExpectations(suite.T())
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleByDeltaError() {

	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	expErr := fmt.Errorf("Unable to scale node")
	logMessage := fmt.Sprintf("scale-nodes error: %s", expErr)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	suite.am.On("Send", "scale_nodes", "mock", requestMessage, "error", expErr.Error()).Return(nil)
	suite.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(0), uint64(0), expErr)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusInternalServerError, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, logMessage)
	suite.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	suite.am.AssertExpectations(suite.T())
	suite.nsm.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "Changed the number of worker nodes on mock from 3 to 4"
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	suite.am.On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	suite.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(3), uint64(4), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusOK, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
	suite.am.AssertExpectations(suite.T())
	suite.nsm.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleNode_ScaleManagerDown() {
	url := "/v1/scale-nodes?type=manager&by=1"
	requestMessage := "Scale nodes down on: mock, by: 1, type: manager"
	message := "Changed the number of manager nodes on mock from 3 to 2"
	jsonStr := `{"groupLabels":{"scale":"down"}}`

	suite.am.On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	suite.nsm.On("ScaleManagerByDelta", -1).Return(uint64(3), uint64(2), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	suite.Equal(http.StatusOK, rec.Code)

	suite.RequireLogs(suite.b.String(), requestMessage, message)
	suite.RequireResponse(rec.Body.Bytes(), "OK", message)
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
