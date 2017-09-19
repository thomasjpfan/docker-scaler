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

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.Nil(err)

	s.Equal(m.Status, "NOK")
	s.Equal(m.Message, message)
	s.Equal(code, rec.Code)
	s.Equal("application/json", rec.HeaderMap["Content-Type"][0])

}

func (s *ResponseTestSuite) Test_ResponseWithJSON() {

	require := s.Require()
	r := Response{Status: "OK", Message: "world"}
	code := http.StatusOK

	rec := httptest.NewRecorder()
	respondWithJSON(rec, code, r)

	var m Response
	err := json.Unmarshal(rec.Body.Bytes(), &m)
	require.Nil(err)

	s.Equal(m.Status, "OK")
	s.Equal(m.Message, "world")
	s.Equal(code, rec.Code)
	s.Equal("application/json", rec.HeaderMap["Content-Type"][0])
}
