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
	scaler  service.ScalerServicer
	alerter service.AlertServicer
	logger  *log.Logger
}

// NewServer creates Server
func NewServer(
	scaler service.ScalerServicer,
	alerter service.AlertServicer,
	logger *log.Logger) *Server {
	return &Server{
		scaler:  scaler,
		alerter: alerter,
		logger:  logger,
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

	requestMessage := fmt.Sprintf("Scale service: %s, delta: %s", serviceID, deltaStr)
	s.logger.Printf(requestMessage)

	if err != nil {
		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	minReplicas, maxReplicas, err := s.scaler.GetMinMaxReplicas(serviceID)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Print(message)
		err := s.alerter.Send("scale_service", serviceID, requestMessage, "error", message)
		if err != nil {
			s.logger.Print(err.Error())
		}
		return
	}

	replicas, err := s.scaler.GetReplicas(serviceID)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	}
	newReplicasInt := int(replicas) + delta

	if newReplicasInt <= 0 {
		message := fmt.Sprintf("Delta %d results in a negative number of replicas for service: %s", delta, serviceID)
		respondWithError(w, http.StatusBadRequest, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	newReplicas := uint64(newReplicasInt)
	if newReplicas > maxReplicas {
		message := fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceID, maxReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	} else if newReplicas < minReplicas {
		message := fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceID, minReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	err = s.scaler.SetReplicas(serviceID, newReplicas)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.sendMessage("scale_service", serviceID, requestMessage, "error", message)
		return
	}
	message := fmt.Sprintf("Scaling %s to %d replicas", serviceID, newReplicas)
	s.sendMessage("scale_service", serviceID, requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

func (s *Server) sendMessage(alertName string, serviceName string, request string,
	status string, message string) {
	s.logger.Print(message)
	err := s.alerter.Send(alertName, serviceName, request, status, message)
	if err != nil {
		s.logger.Print(err.Error())
	}
}
