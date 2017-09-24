package server

import (
	"bytes"
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

func (m *ScalerServicerMock) GetReplicas(serviceName string) (uint64, error) {
	args := m.Called(serviceName)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *ScalerServicerMock) SetReplicas(serviceName string, count uint64) error {
	args := m.Called(serviceName, count)
	return args.Error(0)
}

func (m *ScalerServicerMock) GetMinMaxReplicas(serviceName string) (uint64, uint64, error) {
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

type ServerTestSuite struct {
	suite.Suite
	m  *ScalerServicerMock
	am *AlertServicerMock
	s  *Server
	r  *mux.Router
	l  *log.Logger
	b  *bytes.Buffer
}

func TestServerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (suite *ServerTestSuite) SetupTest() {
	suite.m = new(ScalerServicerMock)
	suite.am = new(AlertServicerMock)
	suite.b = new(bytes.Buffer)
	suite.l = log.New(suite.b, "", 0)
	suite.s = NewServer(suite.m, suite.am, suite.l)
	suite.r = suite.s.MakeRouter()
}

func (suite *ServerTestSuite) Test_NonIntegerDeltaQuery() {

	require := suite.Require()
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		url := fmt.Sprintf("/scale?service=hello&delta=%v", deltaStr)
		requestMessage := fmt.Sprintf("Scale service: hello, delta: %s", deltaStr)
		errorMessage := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		suite.am.On("Send", "scale_service", "hello",
			requestMessage, "error", errorMessage).Return(nil)

		req, _ := http.NewRequest("POST", url, nil)
		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		require.Equal(http.StatusBadRequest, rec.Code)

		logMessages := strings.Split(suite.b.String(), "\n")
		require.Equal(requestMessage, logMessages[0])

		var m Response
		err := json.Unmarshal(rec.Body.Bytes(), &m)
		require.NoError(err)

		require.Equal("NOK", m.Status)
		require.Equal(errorMessage, m.Message)
		require.Equal(errorMessage, logMessages[1])
		suite.am.AssertExpectations(suite.T())
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_DeltaResultsNegativeReplica() {

	require := suite.Require()
	requestMessage := "Scale service: web, delta: -10"
	errorMessage := "Delta -10 results in a negative number of replicas for service: web"
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", errorMessage).Return(nil)

	url := "/scale?service=web&delta=-10"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(errorMessage, m.Message)
	require.Equal(errorMessage, logMessages[1])
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())

}

func (suite *ServerTestSuite) Test_ScaleService_DoesNotExist() {
	require := suite.Require()
	expErr := fmt.Errorf("No such service: web")
	requestMessage := "Scale service: web, delta: 1"
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(0), uint64(0), expErr)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusInternalServerError, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
	require.Equal(expErr.Error(), logMessages[1])
}

func (suite *ServerTestSuite) Test_ScaleService_ScaledToMax() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: 1"
	expErr := fmt.Errorf("web is already scaled to the maximum number of 4 replicas")
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusPreconditionFailed, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
	require.Equal(expErr.Error(), logMessages[1])
}

func (suite *ServerTestSuite) Test_ScaleService_DescaledToMin() {

	require := suite.Require()
	requestMessage := "Scale service: web, delta: -1"
	expErr := fmt.Errorf("web is already descaled to the minimum number of 2 replicas")
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(2), nil)
	suite.am.On("Send", "scale_service", "web",
		requestMessage, "error", expErr.Error()).Return(nil)

	url := "/scale?service=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusPreconditionFailed, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerUp() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: 1"
	message := "Scaling web to 4 replicas"
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(4)).Return(nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("OK", m.Status)
	require.Equal(message, m.Message)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])
	require.Equal(message, logMessages[1])

	suite.m.AssertExpectations(suite.T())
	suite.m.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerDown() {
	require := suite.Require()
	requestMessage := "Scale service: web, delta: -1"
	message := "Scaling web to 2 replicas"

	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(2)).Return(nil)
	suite.am.On("Send", "scale_service", "web", requestMessage, "success", message).Return(nil)

	url := "/scale?service=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("OK", m.Status)
	require.Equal(message, m.Message)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(requestMessage, logMessages[0])
	require.Equal(message, logMessages[1])
	suite.m.AssertExpectations(suite.T())
	suite.am.AssertExpectations(suite.T())
}
