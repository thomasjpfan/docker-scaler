package handler

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryLoggerUnitTest(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", log.LstdFlags)
	errorMsg := "Unexpected error!"

	handler := RecoveryHandler(logger)
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		panic(errorMsg)
	})

	recovery := handler(handlerFunc)
	request, _ := http.NewRequest("GET", "/hello", nil)
	recovery.ServeHTTP(httptest.NewRecorder(), request)

	if !strings.Contains(buf.String(), errorMsg) {
		t.Fatalf("Got log %#v, wanted substring %#v", buf.String(), errorMsg)
	}

}
