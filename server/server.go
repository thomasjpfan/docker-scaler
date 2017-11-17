package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	if r.Body == nil {
		message := "No POST body"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		message := "Unable to decode POST body"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	var ssReq ScaleServiceRequest
	err = json.Unmarshal(body, &ssReq)

	if err != nil {
		message := "Unable to recognize POST body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	serviceName := ssReq.GroupLabels.Service
	scaleDirection := ssReq.GroupLabels.Scale

	if len(serviceName) == 0 {
		message := "No service name in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if len(scaleDirection) == 0 {
		message := "No scale direction in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	requestMessage := fmt.Sprintf("Scale service %s: %s", scaleDirection, serviceName)
	s.logger.Print(requestMessage)

	minReplicas, maxReplicas, err := s.scaler.GetMinMaxReplicas(ctx, serviceName)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}

	scaleDownBy, scaleUpBy, err := s.scaler.GetDownUpScaleDeltas(ctx, serviceName)
	if err != nil {
		message := "Unable to get scaling delta"
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	var delta int
	if scaleDirection == "down" {
		delta = -1 * int(scaleDownBy)
	} else {
		delta = int(scaleUpBy)
	}

	replicas, err := s.scaler.GetReplicas(ctx, serviceName)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}
	newReplicasInt := int(replicas) + delta

	var newReplicas uint64
	if newReplicasInt < int(minReplicas) {
		newReplicas = minReplicas
	} else if newReplicasInt > int(maxReplicas) {
		newReplicas = maxReplicas
	} else {
		newReplicas = uint64(newReplicasInt)
	}

	if replicas == maxReplicas && newReplicas == maxReplicas {
		message := fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceName, maxReplicas)
		respondWithError(w, http.StatusOK, message)
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	} else if replicas == minReplicas && newReplicas == minReplicas {
		message := fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceName, minReplicas)
		respondWithError(w, http.StatusOK, message)
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}

	err = s.scaler.SetReplicas(ctx, serviceName, newReplicas)
	if err != nil {
		message := err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Print(message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}
	message := fmt.Sprintf("Scaling %s from %d to %d replicas", serviceName, replicas, newReplicas)
	s.logger.Print(message)
	s.sendAlert("scale_service", serviceName, requestMessage, "success", message)
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
		s.logger.Print(message)
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", message)
		return
	}

	delta, err := strconv.Atoi(deltaStr)
	if err != nil {
		message := fmt.Sprintf("Incorrect delta query: %v", deltaStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.logger.Print(message)
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", message)
		return
	}

	nodeScaler, err := s.nodeScalerCreater.New(nodesOn)
	if err != nil {
		respondWithError(w, http.StatusPreconditionFailed, err.Error())
		s.logger.Print(err)
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
		s.logger.Print(err)
		s.sendAlert("scale_node", nodesOn, requestMessage, "error", err.Error())
		return
	}

	message := fmt.Sprintf("Changed the number of %s nodes on %s from %d to %d", typeStr, nodesOn, nodesBefore, nodesNow)
	s.logger.Print(message)
	s.sendAlert("scale_node", nodesOn, requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

func (s *Server) sendAlert(alertName string, serviceName string, request string,
	status string, message string) {
	err := s.alerter.Send(alertName, serviceName, request, status, message)
	if err != nil {
		s.logger.Printf("Alertmanager did not receive message: %s, error: %v", message, err)
	} else {
		s.logger.Printf("Alertmanager received message: %s", message)
	}
}
