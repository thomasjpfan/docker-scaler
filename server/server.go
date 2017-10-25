package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/thomasjpfan/docker-scaler/server/handler"

	"github.com/gorilla/mux"
	"github.com/thomasjpfan/docker-scaler/service"
)

// Server runs service that scales docker services
type Server struct {
	scaler            service.ScalerServicer
	alerter           service.AlertServicer
	nodeScaler        service.NodeScaler
	nodeScalerCreater service.NodeScalerCreater
	logger            *log.Logger
}

// NewServer creates Server
func NewServer(
	scaler service.ScalerServicer,
	alerter service.AlertServicer,
	nodeScalerCreater service.NodeScalerCreater,
	logger *log.Logger) *Server {
	return &Server{
		scaler:            scaler,
		alerter:           alerter,
		nodeScalerCreater: nodeScalerCreater,
		logger:            logger,
	}
}

// MakeRouter routes url paths to handlers
func (s *Server) MakeRouter() *mux.Router {
	router := mux.NewRouter()
	v1Router := router.PathPrefix("/v1").Subrouter()
	v1Router.Path("/scale-service").
		Queries("name", "{name}", "delta", "{delta}").
		Methods("POST").
		HandlerFunc(s.ScaleService).
		Name("ScaleService")
	v1Router.Path("/scale-nodes").
		Queries("backend", "{backend}", "delta", "{delta}",
			"type", "{type}").
		Methods("POST").
		HandlerFunc(s.ScaleNode).
		Name("ScaleNode")
	return router
}

// Run starts server
func (s *Server) Run(port uint16) {
	address := fmt.Sprintf(":%d", port)
	m := s.MakeRouter()
	h := handler.RecoveryHandler(s.logger)
	log.Fatal(http.ListenAndServe(address, h(m)))
}

// ScaleService scales service
func (s *Server) ScaleService(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := r.URL.Query()
	serviceID := q.Get("name")
	deltaStr := q.Get("delta")

	requestMessage := fmt.Sprintf("Scale service: %s, delta: %s", serviceID, deltaStr)
	s.logger.Printf(requestMessage)

	delta, err := strconv.Atoi(deltaStr)
	if err != nil {
		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	minReplicas, maxReplicas, err := s.scaler.GetMinMaxReplicas(ctx, serviceID)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Print(message)
		err := s.alerter.Send("scale_service", serviceID, requestMessage, "error", message)
		if err != nil {
			s.logger.Print(message)
		}
		return
	}

	replicas, err := s.scaler.GetReplicas(ctx, serviceID)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	}
	newReplicasInt := int(replicas) + delta

	if newReplicasInt <= 0 {
		message := fmt.Sprintf("Delta %d results in a negative number of replicas for service: %s", delta, serviceID)
		respondWithError(w, http.StatusBadRequest, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	newReplicas := uint64(newReplicasInt)
	if newReplicas > maxReplicas {
		message := fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceID, maxReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	} else if newReplicas < minReplicas {
		message := fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceID, minReplicas)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	}

	err = s.scaler.SetReplicas(ctx, serviceID, newReplicas)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.sendAlert("scale_service", serviceID, requestMessage, "error", message)
		return
	}
	message := fmt.Sprintf("Scaling %s to %d replicas", serviceID, newReplicas)
	s.sendAlert("scale_service", serviceID, requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

// ScaleNode scales nodes
func (s *Server) ScaleNode(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query()
	nodesOn := q.Get("backend")
	deltaStr := q.Get("delta")
	typeStr := q.Get("type")
	ctx := r.Context()

	requestMessage := fmt.Sprintf("Scale node on: %s, delta: %s, type: %s", nodesOn, deltaStr, typeStr)
	s.logger.Printf(requestMessage)

	if typeStr != "worker" && typeStr != "manager" {
		message := fmt.Sprintf("Incorrect type: %s, type can only be worker or manager", typeStr)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", message)
		return
	}

	delta, err := strconv.Atoi(deltaStr)
	if err != nil {
		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", message)
		return
	}

	nodeScaler, err := s.nodeScalerCreater.New(nodesOn)
	if err != nil {
		respondWithError(w, http.StatusPreconditionFailed, err.Error())
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", err.Error())
		return
	}

	var nodesBefore, nodesNow uint64

	if typeStr == "worker" {
		nodesBefore, nodesNow, err = nodeScaler.ScaleWorkerByDelta(ctx, delta)
	} else {
		nodesBefore, nodesNow, err = nodeScaler.ScaleManagerByDelta(ctx, delta)
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", err.Error())
		return
	}

	message := fmt.Sprintf("Changed the number of %s nodes on %s from %d to %d", typeStr, nodesOn, nodesBefore, nodesNow)
	s.sendAlert("scale_node", nodesOn, requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

func (s *Server) sendAlert(alertName string, serviceName string, request string,
	status string, message string) {
	s.logger.Print(message)
	err := s.alerter.Send(alertName, serviceName, request, status, message)
	if err != nil {
		s.logger.Printf("Alertmanager did not receive message: %s, error: %v", message, err)
	} else {
		s.logger.Printf("Alertmanager received message: %s", message)
	}
}
