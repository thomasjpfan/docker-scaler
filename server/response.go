package server

import (
	"encoding/json"
	"net/http"
)

// Response message returns to HTTP clients for scaleing
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	r := Response{Status: "NOK", Message: message}
	respondWithJSON(w, code, r)
}

func respondWithJSON(w http.ResponseWriter, code int, payload Response) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
