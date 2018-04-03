package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/thomasjpfan/docker-scaler/service"
	"github.com/thomasjpfan/docker-scaler/service/cloud"
)

type ScalerServicerMock struct {
	mock.Mock
}

func (m *ScalerServicerMock) Scale(ctx context.Context, serviceName string, by uint64, direction service.ScaleDirection) (string, bool, error) {
	args := m.Called(ctx, serviceName, by, direction)
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

func (nsm *NodeScalerMock) Scale(ctx context.Context, by uint64, direction service.ScaleDirection, nodeType cloud.NodeType, serviceName string) (uint64, uint64, error) {
	args := nsm.Called(ctx, by, direction, nodeType, serviceName)
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

func (rsm *ReschedulerServiceMock) RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error, statusC chan<- string) {
	rsm.Called(manager, targetNodeCnt, value, tickerC, errorC, statusC)
}

func (rsm *ReschedulerServiceMock) RescheduleAll(value string) (string, error) {
	args := rsm.Called(value)
	return args.String(0), args.Error(1)
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
		s.nsm, s.rsm, s.l, false, true, false, true)
	s.r = s.s.MakeRouter("/")
}

func (s *ServerTestSuite) Test_MakeRouter_WithPrefix() {
	m := s.s.MakeRouter("/scaler")
	s.NotNil(m)
}

func (s *ServerTestSuite) Test_Ping_Returns_StatusCode() {
	url := "/v1/ping"
	req, _ := http.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
}

func (s *ServerTestSuite) Test_ScaleService_NoServiceNameInBody() {
	errorMessage := "No service name in request"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"scale": "up"}}`
	logMessage := fmt.Sprintf("scale-service error: %s", errorMessage)
	s.am.On("Send", "scale_service", "bad_request", "Incorrect request", "error", errorMessage).Return(nil)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_NoScaleDirectionInBody() {
	errorMessage := "No scale direction in request"
	url := "/v1/scale-service"
	jsonStr := `{"groupLabels":{"service": "web"}}`
	s.am.On("Send", "scale_service", "bad_request", "Incorrect request", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s", errorMessage)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))

	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.RequireLogs(s.b.String(), logMessage)
	s.am.AssertExpectations(s.T())

}

func (s *ServerTestSuite) Test_ScaleService_IncorrectScaleName() {
	errorMessage := "Incorrect scale direction in request"
	jsonStr := `{"groupLabels":{"service": "web", "scale": "boo"}}`
	s.am.On("Send", "scale_service", "bad_request", "Incorrect request", "error", errorMessage).Return(nil)
	logMessage := fmt.Sprintf("scale-service error: %s", errorMessage)
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
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleUpDirection).Return(expMsg, false, nil)

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

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_Query() {
	requestMessage := "Scale service up: web"
	expMsg := "Scaled up service: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(1), service.ScaleUpDirection).Return(expMsg, false, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service?service=web&scale=up&by=1"

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_NegativeBy_Query() {
	requestMessage := "Scale service up: web"
	expMsg := "Scaled up service: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(2), service.ScaleUpDirection).Return(expMsg, false, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service?service=web&scale=up&by=-2"

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_AlertFails() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	expMsg := "Scaled up service: web"
	alertErr := errors.New("Alert failed")
	alertMsg := fmt.Sprintf("Alertmanager did not receive message: %s, error: %v", expMsg, alertErr)
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(alertErr)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleUpDirection).Return(expMsg, false, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage, alertMsg)
	s.am.AssertExpectations(s.T())
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_Bounded_AlertMaxTrue() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	expMsg := "web is already scaled to the maximum number of 5 replicas"
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleUpDirection).Return(expMsg, true, nil)

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

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_Bounded_AlertMaxFalse() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	expMsg := "web is already scaled to the maximum number of 5 replicas"
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleUpDirection).Return(expMsg, true, nil)
	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)

	url := "/v1/scale-service"
	ser := NewServer(s.m, s.am,
		s.nsm, s.rsm, s.l, false, false, false, true)
	serRouter := ser.MakeRouter("/")

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()

	serRouter.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertNotCalled(s.T(), "Send", "scale_service", "web", requestMessage, "success", expMsg)
	s.m.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_Bounded_AlertMinTrue() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	expMsg := "web is already descaled to the minimum number of 1 replicas"

	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleDownDirection).Return(expMsg, true, nil)
	s.am.On("Send", "scale_service", "web", requestMessage, "success", expMsg).Return(nil)
	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)

	url := "/v1/scale-service"
	ser := NewServer(s.m, s.am,
		s.nsm, s.rsm, s.l, true, true, false, true)
	serRouter := ser.MakeRouter("/")

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()

	serRouter.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.m.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_Bounded_AlertMinFalse() {
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	expMsg := "web is already descaled to the minimum number of 1 replicas"
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleDownDirection).Return(expMsg, true, nil)
	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)

	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()

	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.m.AssertExpectations(s.T())
	s.am.AssertNotCalled(s.T(), "Send", "scale_service", "web", requestMessage, "success", expMsg)
}

func (s *ServerTestSuite) Test_ScaleService_ScaleUp_Error() {
	expErr := fmt.Errorf("Unable to scale service: web")
	jsonStr := `{"groupLabels":{"service": "web", "scale": "up"}}`
	requestMessage := "Scale service up: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "error", expErr.Error()).Return(nil)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleUpDirection).Return("", false, expErr)

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
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleDownDirection).Return(expMsg, false, nil)

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
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleDownDirection).Return(expMsg, true, nil)

	logMessage := fmt.Sprintf("scale-service success: %s", expMsg)
	url := "/v1/scale-service"

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", expMsg)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.am.AssertNotCalled(s.T(), "Send", "scale_service", "web", requestMessage, "success", expMsg)
	s.m.AssertExpectations(s.T())
	s.Len(s.am.Calls, 0)
}

func (s *ServerTestSuite) Test_ScaleService_ScaleDown_Error() {
	expErr := fmt.Errorf("Unable to scale service: web")
	jsonStr := `{"groupLabels":{"service": "web", "scale": "down"}}`
	requestMessage := "Scale service down: web"
	s.am.On("Send", "scale_service", "web", requestMessage, "error", expErr.Error()).Return(nil)
	s.m.On("Scale", mock.AnythingOfType("*context.valueCtx"), "web", uint64(0), service.ScaleDownDirection).Return("", false, expErr)

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

func (s *ServerTestSuite) Test_ScaleNode_Nil_NodeScaler() {
	server := NewServer(s.m, s.am,
		nil, s.rsm, s.l, false, true, false, true)
	router := server.MakeRouter("/")

	url := "/v1/scale-nodes"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusNotFound, rec.Code)
}

func (s *ServerTestSuite) Test_ScaleNode_NoScaleDirectionInBody() {
	errorMessage := "No scale direction"
	url := "/v1/scale-nodes?by=1&type=worker"
	jsonStr := `{"groupLabels":{}}`
	logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
	s.am.On("Send", "scale_nodes", "bad_request", "Incorrect request", "error", errorMessage).Return(nil)
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

		errorMessage := "Incorrect scale direction"
		url := "/v1/scale-nodes?by=1&type=worker"
		jsonStr := fmt.Sprintf(`{"groupLabels":{"scale":"%s"}}`, deltaStr)
		logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
		s.am.On("Send", "scale_nodes", "bad_request", "Incorrect request", "error", errorMessage).Return(nil)
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
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeWorkerType, "").Return(uint64(0), uint64(0), expErr)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusInternalServerError, rec.Code)

	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.RequireResponse(rec.Body.Bytes(), "NOK", expErr.Error())
	s.am.AssertExpectations(s.T())
	s.nsm.AssertExpectations(s.T())

}

func (s *ServerTestSuite) Test_ScaleNode_IncorrectNodeType() {

	url := "/v1/scale-nodes?type=invalid&by=1"
	errorMessage := "Incorrect node type: invalid, type can only be worker or manager"
	logMessage := fmt.Sprintf("scale-nodes error: %s", errorMessage)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.am.On("Send", "scale_nodes", "mock", "Incorrect request", "error", errorMessage).Return(nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)

	s.RequireLogs(s.b.String(), logMessage)
	s.RequireResponse(rec.Body.Bytes(), "NOK", errorMessage)
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "Changing the number of worker nodes on mock from 3 to 4"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	rescheduleMsg := "Waiting for worker nodes to scale from 3 to 4 for rescheduling"
	logMessage2 := fmt.Sprintf("scale-nodes: %s", rescheduleMsg)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	done := make(chan struct{})
	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil).
		On("Send", "scale_nodes", "reschedule", "Wait to reschedule", "pending", rescheduleMsg).Return(nil).
		On("Send", "reschedule_service", "reschedule", "Waiting for nodes to scale", "error", mock.AnythingOfType("string")).Return(nil).
		On("Send", "reschedule_service", "reschedule", "Waiting for nodes to scale", "pending", mock.AnythingOfType("string")).Return(nil).
		On("Send", "reschedule_service", "reschedule", "4 worker nodes are online, status: web_test rescheduled", "success", "4 worker nodes are online, status: web_test rescheduled").Return(nil).Run(func(args mock.Arguments) {
		done <- struct{}{}
	})

	var tickerC chan<- time.Time
	var errC chan<- error
	var statusC chan<- string
	waitCalled := make(chan struct{})
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeWorkerType, "").Return(uint64(3), uint64(4), nil)
	s.rsm.On("RescheduleServicesWaitForNodes", false, 4, mock.AnythingOfType("string"),
		mock.AnythingOfType("chan<- time.Time"), mock.AnythingOfType("chan<- error"), mock.AnythingOfType("chan<- string")).Return().Run(func(args mock.Arguments) {
		tickerC = args.Get(3).(chan<- time.Time)
		errC = args.Get(4).(chan<- error)
		statusC = args.Get(5).(chan<- string)
		waitCalled <- struct{}{}
	})

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
	s.RequireResponse(rec.Body.Bytes(), "OK", message)

H:
	for {
		select {
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-waitCalled:
			break H
		}
	}
	go func() {
		tickerC <- time.Now()
		errC <- errors.New("Here is an error")
		statusC <- "web_test rescheduled"
	}()

L:
	for {
		select {
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-done:
			break L
		}
	}

	s.RequireLogs(s.b.String(), requestMessage, logMessage, logMessage2)
	logMessages := strings.Split(s.b.String(), "\n")
	s.Len(logMessages, 7)

	s.rsm.AssertExpectations(s.T())
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleManagerDown() {
	url := "/v1/scale-nodes?type=manager&by=1"
	requestMessage := "Scale nodes down on: mock, by: 1, type: manager"
	message := "Changing the number of manager nodes on mock from 3 to 2"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"down"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleDownDirection, cloud.NodeManagerType, "").Return(uint64(3), uint64(2), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleManagerDown_QueryInBody() {
	url := "/v1/scale-nodes"
	requestMessage := "Scale nodes down on: mock, by: 1, type: manager"
	message := "Changing the number of manager nodes on mock from 3 to 2"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"down","type":"manager","by":1}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleDownDirection, cloud.NodeManagerType, "").Return(uint64(3), uint64(2), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerDown_QueryInBodyWithServiceTrigger() {
	url := "/v1/scale-nodes?by=1&type=worker"
	requestMessage := "Scale nodes down on: mock, by: 1, type: worker"
	message := "Changing the number of worker nodes on mock from 3 to 2"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"down","service":"node_exporter"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleDownDirection, cloud.NodeWorkerType, "node_exporter").Return(uint64(3), uint64(2), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp_QueryInURL() {
	url := "/v1/scale-nodes?type=worker&by=1&scale=up&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "Changing the number of worker nodes on mock from 3 to 4"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil).
		On("Send", "scale_nodes", "reschedule", "Wait to reschedule", "pending", mock.AnythingOfType("string")).Return(nil)

	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeWorkerType, "").Return(uint64(3), uint64(4), nil)

	waitCalled := make(chan struct{})
	s.rsm.On("RescheduleServicesWaitForNodes", false, 4, mock.AnythingOfType("string"),
		mock.AnythingOfType("chan<- time.Time"), mock.AnythingOfType("chan<- error"), mock.AnythingOfType("chan<- string")).Return().Run(func(args mock.Arguments) {
		waitCalled <- struct{}{}
	})

	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

L:
	for {
		select {
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
		case <-waitCalled:
			break L
		}
	}

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
	s.rsm.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleManagerUp_MaxAlertOn() {
	url := "/v1/scale-nodes?type=manager&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: manager"
	message := "manager nodes are already scaled to the maximum number of 4 nodes"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeManagerType, "").Return(uint64(4), uint64(4), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp_MaxAlertOn() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "worker nodes are already scaled to the maximum number of 3 nodes"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeWorkerType, "").Return(uint64(3), uint64(3), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerUp_MaxAlertFalse() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes up on: mock, by: 1, type: worker"
	message := "worker nodes are already scaled to the maximum number of 3 nodes"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"up"}}`

	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleUpDirection, cloud.NodeWorkerType, "").Return(uint64(3), uint64(3), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	ser := NewServer(s.m, s.am,
		s.nsm, s.rsm, s.l, false, true, false, false)
	serRouter := ser.MakeRouter("/")

	serRouter.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertNotCalled(s.T(), "Send", "scale_nodes", "mock", requestMessage, "success", message)
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleWorkerDown_MinAlertTrue() {
	url := "/v1/scale-nodes?type=worker&by=1"
	requestMessage := "Scale nodes down on: mock, by: 1, type: worker"
	message := "worker nodes are already descaled to the minimum number of 1 nodes"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"down"}}`

	s.am.
		On("Send", "scale_nodes", "mock", requestMessage, "success", message).Return(nil)
	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleDownDirection, cloud.NodeWorkerType, "").Return(uint64(1), uint64(1), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	ser := NewServer(s.m, s.am,
		s.nsm, s.rsm, s.l, false, true, true, true)
	serRouter := ser.MakeRouter("/")

	serRouter.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertExpectations(s.T())
}

func (s *ServerTestSuite) Test_ScaleNode_ScaleManagerDown_MinAlertFalse() {
	url := "/v1/scale-nodes?type=manager&by=1"
	requestMessage := "Scale nodes down on: mock, by: 1, type: manager"
	message := "manager nodes are already descaled to the minimum number of 1 nodes"
	logMessage := fmt.Sprintf("scale-nodes success: %s", message)
	jsonStr := `{"groupLabels":{"scale":"down"}}`

	s.nsm.On("Scale", mock.AnythingOfType("*context.valueCtx"), uint64(1), service.ScaleDownDirection, cloud.NodeManagerType, "").Return(uint64(1), uint64(1), nil)

	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(jsonStr))
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	s.RequireResponse(rec.Body.Bytes(), "OK", message)
	s.RequireLogs(s.b.String(), requestMessage, logMessage)
	s.nsm.AssertExpectations(s.T())
	s.am.AssertNotCalled(s.T(), "Send", "scale_nodes", "mock", requestMessage, "success", message)
}

func (s *ServerTestSuite) Test_RescheduleAllServicesError() {
	url := "/v1/reschedule-services"
	requestMessage := "Rescheduling all labeled services"
	expErr := fmt.Errorf("Unable to reschedule service")
	logMessage := fmt.Sprintf("reschedule-services error: %s", expErr)

	s.rsm.On("RescheduleAll", mock.AnythingOfType("string")).Return("", expErr)
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

func (s *ServerTestSuite) Test_RescheduleAllServices() {
	url := "/v1/reschedule-services"
	requestMessage := "Rescheduling all labeled services"
	message := "Rescheduled services web_test"
	logMessage := fmt.Sprintf("reschedule-services success: %s", message)

	s.rsm.On("RescheduleAll", mock.AnythingOfType("string")).Return(message, nil)
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
