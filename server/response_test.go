package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ResponseTestSuite struct {
	suite.Suite
}

func TestResponseUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ResponseTestSuite))
}

func (s *ResponseTestSuite) Test_RespondWithError() {
	require := s.Require()
	code := http.StatusBadRequest
	message := "ERROR ERROR"

	rec := httptest.NewRecorder()
	respondWithError(rec, code, message)

	var m map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.Nil(err)

	s.Equal(m["error"], message)
	s.Equal(code, rec.Code)
	s.Equal("application/json", rec.HeaderMap["Content-Type"][0])

}

func (s *ResponseTestSuite) Test_ResponseWithJSON() {

	require := s.Require()
	payload := map[string]string{"hello": "world"}
	code := http.StatusOK

	rec := httptest.NewRecorder()
	respondWithJSON(rec, code, payload)

	var m map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.Nil(err)

	s.Equal(m["hello"], "world")
	s.Equal(code, rec.Code)
	s.Equal("application/json", rec.HeaderMap["Content-Type"][0])
}
