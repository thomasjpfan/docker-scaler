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

type ServerTestSuite struct {
	suite.Suite
	m *ScalerServicerMock
	s *Server
	r *mux.Router
	l *log.Logger
	b *bytes.Buffer
}

func TestServerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (suite *ServerTestSuite) SetupTest() {
	suite.m = new(ScalerServicerMock)
	suite.b = new(bytes.Buffer)
	suite.l = log.New(suite.b, "", 0)
	suite.s = NewServer(suite.m, suite.l)
	suite.r = suite.s.MakeRouter()
}

func (suite *ServerTestSuite) Test_NonIntegerDeltaQuery() {

	require := suite.Require()
	tt := []string{"what", "2114what", "24y4", "he"}

	for _, deltaStr := range tt {

		url := fmt.Sprintf("/scale?service=hello&delta=%v", deltaStr)
		req, _ := http.NewRequest("POST", url, nil)

		rec := httptest.NewRecorder()
		suite.r.ServeHTTP(rec, req)
		require.Equal(http.StatusBadRequest, rec.Code)

		logMessages := strings.Split(suite.b.String(), "\n")
		require.Equal(logMessages[0], "Request to scale service: hello")

		var m Response
		err := json.Unmarshal(rec.Body.Bytes(), &m)
		require.NoError(err)

		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		require.Equal("NOK", m.Status)
		require.Equal(message, m.Message)
		require.Equal(message, logMessages[1])
		suite.b.Reset()
	}
}

func (suite *ServerTestSuite) Test_DeltaResultsNegativeReplica() {

	require := suite.Require()
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)

	url := "/scale?service=web&delta=-10"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusBadRequest, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal("Request to scale service: web", logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	message := fmt.Sprintf("Delta -10 results in a negative number of replicas for service: web")
	require.Equal("NOK", m.Status)
	require.Equal(message, m.Message)
	require.Equal(message, logMessages[1])

}

func (suite *ServerTestSuite) Test_ScaleService_DoesNotExist() {
	require := suite.Require()
	expErr := fmt.Errorf("No such service: web")
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(0), uint64(0), expErr)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusInternalServerError, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal("Request to scale service: web", logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
	require.Equal(expErr.Error(), logMessages[1])
}

func (suite *ServerTestSuite) Test_ScaleService_ScaledToMax() {
	require := suite.Require()
	expErr := fmt.Errorf("web is already scaled to the maximum number of 4 replicas")
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(1), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(4), nil)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusPreconditionFailed, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal("Request to scale service: web", logMessages[0])

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
	require.Equal(expErr.Error(), logMessages[1])
}

func (suite *ServerTestSuite) Test_ScaleService_DescaledToMin() {

	require := suite.Require()
	expErr := fmt.Errorf("web is already descaled to the minimum number of 2 replicas")
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(2), nil)

	url := "/scale?service=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusPreconditionFailed, rec.Code)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal(logMessages[0], "Request to scale service: web")

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("NOK", m.Status)
	require.Equal(expErr.Error(), m.Message)
	suite.m.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerUp() {
	require := suite.Require()
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(4)).Return(nil)

	url := "/scale?service=web&delta=1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("OK", m.Status)
	require.Equal("Scaling web to 4 replicas", m.Message)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal("Request to scale service: web", logMessages[0])
	require.Equal("Scaling web to 4 replicas", logMessages[1])

	suite.m.AssertExpectations(suite.T())
}

func (suite *ServerTestSuite) Test_ScaleService_CallsScalerServicerDown() {
	require := suite.Require()
	suite.m.On("GetMinMaxReplicas", "web").Return(uint64(2), uint64(4), nil)
	suite.m.On("GetReplicas", "web").Return(uint64(3), nil)
	suite.m.On("SetReplicas", "web", uint64(2)).Return(nil)

	url := "/scale?service=web&delta=-1"
	req, _ := http.NewRequest("POST", url, nil)
	rec := httptest.NewRecorder()
	suite.r.ServeHTTP(rec, req)
	require.Equal(http.StatusOK, rec.Code)

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.NoError(err)

	require.Equal("OK", m.Status)
	require.Equal("Scaling web to 2 replicas", m.Message)

	logMessages := strings.Split(suite.b.String(), "\n")
	require.Equal("Request to scale service: web", logMessages[0])
	require.Equal("Scaling web to 2 replicas", logMessages[1])
	suite.m.AssertExpectations(suite.T())
}
