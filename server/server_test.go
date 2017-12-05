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
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScalerServicerMock struct {
	mock.Mock
}

func (m *ScalerServicerMock) ScaleUp(ctx context.Context, serviceName string) (string, bool, error) {
	args := m.Called(ctx, serviceName)
	return args.String(0), args.Bool(1), args.Error(2)
}

func (m *ScalerServicerMock) ScaleDown(ctx context.Context, serviceName string) (string, bool, error) {
	args := m.Called(ctx, serviceName)
	return args.String(0), args.Bool(1), args.Error(2)
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

type ReschedulerServiceMock struct {
	mock.Mock
}

func (rsm *ReschedulerServiceMock) RescheduleService(serviceID, value string) error {
	args := rsm.Called(serviceID, value)
	return args.Error(0)
}

func (rsm *ReschedulerServiceMock) RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error) {
	rsm.Called(manager, targetNodeCnt, value, tickerC, errorC)
}

func (rsm *ReschedulerServiceMock) RescheduleAll(value string) error {
	args := rsm.Called(value)
	return args.Error(0)
}

type ServerTestSuite struct {
	suite.Suite
	m   *ScalerServicerMock
	am  *AlertServicerMock
	nsm *NodeScalerMock
	rsm *ReschedulerServiceMock
	s   *Server
	r   *mux.Router
	l   *log.Logger
	b   *bytes.Buffer
}

func TestServerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (s *ServerTestSuite) SetupTest() {
	s.m = new(ScalerServicerMock)
	s.am = new(AlertServicerMock)
	s.nsm = new(NodeScalerMock)
	s.rsm = new(ReschedulerServiceMock)

	s.b = new(bytes.Buffer)
	s.l = log.New(s.b, "", 0)
	s.s = NewServer(s.m, s.am,
		s.nsm, s.rsm, s.l)
	s.r = s.s.MakeRouter("/")
}

func (s *ServerTestSuite) Test_ScaleService_NoBody() {
	errorMessage := "No POST body"
	logMessage := fmt.Sprintf("scale-service error: %s", errorMessage)
	url := "/v1/scale-service"
	s.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_NoServiceNameInBody() {
	errorMessage := "No service name in request body"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"scale": "up"}}`
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)
	s.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_NoScaleDirectionInBody() {
	errorMessage := "No scale direction in request body"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"service": "web"}}`
	s.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())

}

func (s *ServerTestSuite) Test_ScaleService_IncorrectScaleName() {
	errorMessage := "Incorrect scale direction in request body"
	jsonStr := `{"groupLabels":{"service": "web", "scale": "boo"}}`
	s.am.On("Send", "scale_service", "bad_request", "", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s, body: %s", errorMessage, jsonStr)
	url := "/v1/scale-service"
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleUp() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	expMsg := "Scaled up service: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("ScaleUp", mock.AnythingOfType("*context.valueCtx"), "web").Return(expMsg, false, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_Error() {
	expErr := fmt.Errorf("Unable to scale service: web")
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "error", expErr.Error()).Return(nil)
	s.m.On("ScaleUp", mock.AnythingOfType("*context.valueCtx"), "web").Return("", false, expErr)

	logMessage := fmt.Sprintf("scale-service error: %s", expErr.Error())
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	expMsg := "Scaled down service: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("ScaleDown", mock.AnythingOfType("*context.valueCtx"), "web").Return(expMsg, false, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_Bounded() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	expMsg := "Scaled down service: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("ScaleDown", mock.AnythingOfType("*context.valueCtx"), "web").Return(expMsg, true, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.m.AssertExpectations(s.T())
	s.Len(s.am.Calls, 0)
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_Error() {
	expErr := fmt.Errorf("Unable to scale service: web")
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "error", expErr.Error()).Return(nil)
	s.m.On("ScaleDown", mock.AnythingOfType("*context.valueCtx"), "web").Return("", false, expErr)

	logMessage := fmt.Sprintf("scale-service error: %s", expErr.Error())
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusInternalServerError, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_NoPOSTBody() {
	errorMessage := "No POST body"
	logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
	url := "/v1/scale-nodes?by=1&type=worker"
	s.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_NoScaleDirectionInBody() {
	errorMessage := "No scale direction in request body"
	url := "/v1/scale-nodes?by=1&type=worker"
	jsonStr := `{"groupLabels":{}}`
	logMessage := fmt.Sprintf("scale-nodes error: %s, body: %s", errorMessage, jsonStr)
	s.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_InvalidScaleValue() {
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		errorMessage := "Incorrect scale direction in request body"
		url := "/v1/scale-nodes?by=1&type=worker"
		jsonStr := fmt.Sprintf(`{"groupLabels":{"scale":"%s"}}`, deltaStr)
		logMessage := fmt.Sprintf("scale-nodes error: %s, body: %s", errorMessage, jsonStr)
		s.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
		req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

		rec := httptest.NewRecorder()
		s.r.ServeHTTP(rec, req)
		s.Equal(http.StatusBadRequest, rec.Code)
		s.RequireLogs(s.b.String(), logMessage)
		s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		s.am.AssertExpectations(s.T())
		s.b.Reset()
	}
}

func (s *ServerTestSuite) Test_ScaleNode_NonIntegerByQuery() {

	tt := []string{"what", "2114what", "24y4", "he"}

	for _, byStr := range tt {

		errorMessage := fmt.Sprintf("Non integer by query parameter: %s", byStr)
		url := fmt.Sprintf("/v1/scale-nodes?by=%s&type=worker", byStr)
		jsonStr := `{"groupLabels":{"scale":"up"}}`
		logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
		s.am.On("Send", "scale_nodes", "bad_request", "", "error", errorMessage).Return(nil)
		req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

		rec := httptest.NewRecorder()
		s.r.ServeHTTP(rec, req)
		s.Equal(http.StatusBadRequest, rec.Code)

		s.RequireLogs(s.b.String(), logMessage)
		s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
		s.am.AssertExpectations(s.T())
		s.b.Reset()
	}
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleByDeltaError() {

	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	expErr := fmt.Errorf("Unable to scale node")
	logMessage := fmt.Sprintf("scale-nodes error: %s", expErr)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.am.On("Send", "scale_nodes", "mock", requestMessage, "error", expErr.Error()).Return(nil)
	s.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(0), uint64(0), expErr)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusInternalServerError, rec.Code)

	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.am.AssertExpectations(s.T())
	s.nsm.AssertExpectations(s.T())

}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "Changing the number of worker nodes on mock from 3 to 4"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	rescheduleMsg := "Waiting for worker nodes to scale from 3 to 4 for rescheduling"
	logMessage2 := fmt.Sprintf("scale-nodes: %s", rescheduleMsg)
	alertMsg := fmt.Sprintf("Alertmanager received message: %s", message)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil).
		On("Send", "scale_nodes", "reschedule", "", "success", rescheduleMsg).Return(nil)

	s.nsm.On("ScaleWorkerByDelta", 1).Return(uint64(3), uint64(4), nil)
	s.rsm.On("RescheduleServicesWaitForNodes", false, 4, mock.AnythingOfType("string"),
		mock.AnythingOfType("chan<- time.Time"), mock.AnythingOfType("chan<- error")).Return()

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage, alertMsg, logMessage2)
	s.nsm.AssertExpectations(s.T())

	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleManagerDown() {
	url := "/v1/scale-nodes?type=manager&by=1"
	requestMessage := "Scale nodes down on: mock, by: 1, type: manager"
	message := "Changing the number of manager nodes on mock from 3 to 2"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	alertMsg := fmt.Sprintf("Alertmanager received message: %s", message)
	rescheduleMsg := "Waiting for manager nodes to scale from 3 to 2 for rescheduling"
	logMessage2 := fmt.Sprintf("scale-nodes: %s", rescheduleMsg)
	jsonStr := `{"groupLabels":{"scale":"down"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil).
		On("Send", "scale_nodes", "reschedule", "", "success", rescheduleMsg).Return(nil)
	s.nsm.On("ScaleManagerByDelta", -1).Return(uint64(3), uint64(2), nil)
	s.rsm.On("RescheduleServicesWaitForNodes", true, 2, mock.AnythingOfType("string"),
		mock.AnythingOfType("chan<- time.Time"), mock.AnythingOfType("chan<- error")).Return()

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage, alertMsg, logMessage2)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_RescheduleAllServicesError() {
	url := "/v1/reschedule-services"
	requestMessage := "Rescheduling all labeled services"
	expErr := fmt.Errorf("Unable to reschedule service")
	logMessage := fmt.Sprintf("reschedule-services error: %s", expErr)

	s.rsm.On("RescheduleAll", mock.AnythingOfType("string")).Return(expErr)
	s.am.On("Send", "reschedule_services", "reschedule", requestMessage, "error", expErr.Error()).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusInternalServerError, rec.Code)

	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.rsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_RescheduleAllServices() {
	url := "/v1/reschedule-services"
	requestMessage := "Rescheduling all labeled services"
	message := "Rescheduled all services"
	logMessage := fmt.Sprintf("reschedule-services success: %s", message)

	s.rsm.On("RescheduleAll", mock.AnythingOfType("string")).Return(nil)
	s.am.On("Send", "reschedule_services", "reschedule", requestMessage, "success", message).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.rsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_RescheduleOneServiceError() {
	url := "/v1/reschedule-service?service=web"
	requestMessage := "Rescheduling service: web"
	expErr := fmt.Errorf("Unable to reschedule service")
	logMessage := fmt.Sprintf("reschedule-service error: %s", expErr)

	s.rsm.On("RescheduleService", "web", mock.AnythingOfType("string")).Return(expErr)
	s.am.On("Send", "reschedule_service", "reschedule", requestMessage, "error", expErr.Error()).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.rsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_RescheduleOneService() {
	url := "/v1/reschedule-service?service=web"
	requestMessage := "Rescheduling service: web"
	message := "Rescheduled service: web"
	logMessage := fmt.Sprintf("reschedule_service success: %s", message)

	s.rsm.On("RescheduleService", "web", mock.AnythingOfType("string")).Return(nil)
	s.am.On("Send", "reschedule_service", "reschedule", requestMessage, "success", message).Return(nil)

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.rsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())

}

func (s *ServerTestSuite) RequireLogs(logMessage string, expectedLogs ...string) {
	logMessages := strings.Split(logMessage, "\n")
	s.Require().True(len(logMessages) >= len(expectedLogs))
	for i, m := range expectedLogs {
		s.Equal(m, logMessages[i])
	}
}

func (s *ServerTestSuite) RequireResponse(data []byte, status string, message string) {
	var m Response
	err := json.Unmarshal(data, &m)
	s.Require().NoError(err)

	s.Equal(status, m.Status)
	s.Equal(message, m.Message)
}
