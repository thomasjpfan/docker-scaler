package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/thomasjpfan/docker-scaler/service"
)

// Server runs service that scales docker services
type Server struct {
	scaler service.ScalerServicer
	logger *log.Logger
}

// NewServer creates Server
func NewServer(
	scaler service.ScalerServicer,
	logger *log.Logger) *Server {
	return &Server{
		scaler: scaler,
		logger: logger,
	}
}

// MakeRouter routes url paths to handlers
func (s *Server) MakeRouter() *mux.Router {
	m := mux.NewRouter()
	m.Path("/scale").
		Queries("service", "{service}", "delta", "{delta}").
		Methods("POST").
		HandlerFunc(s.ScaleService).
		Name("ScaleService")
	return m
}

// Run starts server
func (s *Server) Run(port uint16) {
	address := fmt.Sprintf(":%d", port)
	m := s.MakeRouter()
	log.Fatal(http.ListenAndServe(address, m))
}

// ScaleService scales service
func (s *Server) ScaleService(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query()
	serviceID := q.Get("service")
	deltaStr := q.Get("delta")
	delta, err := strconv.Atoi(deltaStr)

	s.logger.Printf("Request to scale service: %s", serviceID)

	if err != nil {
		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.logger.Print(message)
		return
	}

	// Get scaled service
	minReplicas, maxReplicas, err := s.scaler.GetMinMaxReplicas(serviceID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		s.logger.Print(err.Error())
		return
	}

	replicas, err := s.scaler.GetReplicas(serviceID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		s.logger.Print(err.Error())
		return
	}
	newReplicasInt := int(replicas) + delta

	if newReplicasInt <= 0 {
		message := fmt.Sprintf("Delta %d results in a negative number of replicas for service: %s", delta, serviceID)
		respondWithError(w, http.StatusBadRequest, message)
		s.logger.Print(message)
		return
	}

	newReplicas := uint64(newReplicasInt)
	if newReplicas > maxReplicas {
		message := fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceID, maxReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.logger.Print(message)
		return
	} else if newReplicas < minReplicas {
		message := fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceID, minReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		return
	}

	err = s.scaler.SetReplicas(serviceID, newReplicas)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	message := fmt.Sprintf("Scaling web to %d replicas", newReplicas)
	s.logger.Print(message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}
