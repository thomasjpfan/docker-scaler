package handler

import (
	"log"
	"net/http"
)

type recoveryHandler struct {
	handler http.Handler
	logger  *log.Logger
}

func (h recoveryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("Internal error: %v", err)
		}
	}()

	h.handler.ServeHTTP(w, req)
}

// RecoveryHandler is a HTTP middleware that recovers from a panic,
// logs the panic, and writes http.StatusInternalServerError
func RecoveryHandler(logger *log.Logger) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		r := &recoveryHandler{handler: h, logger: logger}
		return r
	}
}
