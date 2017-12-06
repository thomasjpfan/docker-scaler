package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/thomasjpfan/docker-scaler/server/handler"

	"github.com/gorilla/mux"
	"github.com/thomasjpfan/docker-scaler/service"
)

// Server runs service that scales docker services
type Server struct {
	scaler      service.ScalerServicer
	alerter     service.AlertServicer
	nodeScaler  service.NodeScaler
	rescheduler service.ReschedulerServicer
	logger      *log.Logger
}

// NewServer creates Server
func NewServer(
	scaler service.ScalerServicer,
	alerter service.AlertServicer,
	nodeScaler service.NodeScaler,
	rescheduler service.ReschedulerServicer,
	logger *log.Logger) *Server {
	return &Server{
		scaler:      scaler,
		alerter:     alerter,
		nodeScaler:  nodeScaler,
		rescheduler: rescheduler,
		logger:      logger,
	}
}

// MakeRouter routes url paths to handlers
func (s *Server) MakeRouter(prefix string) *mux.Router {
	router := mux.NewRouter()
	rootPrefix := path.Join(prefix, "/v1")
	v1Router := router.PathPrefix(rootPrefix).Subrouter()
	v1Router.Path("/scale-service").
		Methods("POST").
		HandlerFunc(s.ScaleService).
		Name("ScaleService")
	v1Router.Path("/scale-nodes").
		Queries("type", "{type}", "by", "{by}").
		Methods("POST").
		HandlerFunc(s.ScaleNodes).
		Name("ScaleNode")
	v1Router.Path("/reschedule-services").
		Methods("POST").
		HandlerFunc(s.RescheduleAllServices).
		Name("RescheduleAllServices")
	v1Router.Path("/reschedule-service").
		Methods("POST").
		Queries("service", "{service}").
		HandlerFunc(s.RescheduleOneService).
		Name("RescheduleOneService")
	return router
}

// Run starts server
func (s *Server) Run(port uint16, prefix string) {
	address := fmt.Sprintf(":%d", port)
	m := s.MakeRouter(prefix)
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

	var ssReq ScaleRequest
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

	var message string
	var isBounded bool
	if scaleDirection == "down" {
		message, isBounded, err = s.scaler.ScaleDown(ctx, serviceName)
	} else {
		message, isBounded, err = s.scaler.ScaleUp(ctx, serviceName)
	}

	if err != nil {
		message = err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}

	s.logger.Printf("scale-service success: %s", message)
	if !isBounded {
		s.sendAlert("scale_service", serviceName, requestMessage, "success", message)
	}
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

// ScaleNodes scales nodes
func (s *Server) ScaleNodes(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	q := r.URL.Query()
	typeStr := q.Get("type")
	byStr := q.Get("by")

	if r.Body == nil {
		message := "No POST body"
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	byInt, err := strconv.Atoi(byStr)

	if err != nil {
		message := fmt.Sprintf("Non integer by query parameter: %s", byStr)
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		message := "Unable to decode POST body"
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	var ssReq ScaleRequest
	err = json.Unmarshal(body, &ssReq)

	if err != nil {
		message := "Unable to recognize POST body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	scaleDirection := ssReq.GroupLabels.Scale

	if len(scaleDirection) == 0 {
		message := "No scale direction in request body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction in request body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection == "down" {
		byInt *= -1
	}

	requestMessage := fmt.Sprintf("Scale nodes %s on: %s, by: %s, type: %s", scaleDirection, s.nodeScaler, byStr, typeStr)
	s.logger.Printf(requestMessage)

	if typeStr != "worker" && typeStr != "manager" {
		message := fmt.Sprintf("Incorrect type: %s, type can only be worker or manager", typeStr)
		respondWithError(w, http.StatusPreconditionFailed, message)
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", fmt.Sprint(s.nodeScaler), requestMessage, "error", message)
		return
	}

	var nodesBefore, nodesNow uint64

	isManager := (typeStr == "manager")

	if isManager {
		nodesBefore, nodesNow, err = s.nodeScaler.ScaleManagerByDelta(ctx, byInt)
	} else {
		nodesBefore, nodesNow, err = s.nodeScaler.ScaleWorkerByDelta(ctx, byInt)
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		s.logger.Printf("scale-nodes error: %s", err)
		s.sendAlert("scale_nodes", fmt.Sprint(s.nodeScaler), requestMessage, "error", err.Error())
		return
	}

	message := fmt.Sprintf("Changing the number of %s nodes on %s from %d to %d", typeStr, s.nodeScaler, nodesBefore, nodesNow)
	s.logger.Printf("scale-nodes success: %s", message)
	s.sendAlert("scale_nodes", fmt.Sprint(s.nodeScaler), requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})

	// Call rescheduler if nodesNow is greater than nodesBefore

	if nodesNow > nodesBefore {
		rightNow := time.Now().UTC().Format("20060102T150405")
		reqMsg := fmt.Sprintf("Waiting for %s nodes to scale from %d to %d for rescheduling", typeStr, nodesBefore, nodesNow)
		s.logger.Printf("scale-nodes: %s", reqMsg)
		s.sendAlert("scale_nodes", "reschedule", "", "success", reqMsg)

		go s.rescheduleServiceWait(isManager, int(nodesNow), rightNow)
	}
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

// RescheduleAllServices reschedules all services
func (s *Server) RescheduleAllServices(w http.ResponseWriter, r *http.Request) {
	requestMessage := "Rescheduling all labeled services"
	s.logger.Print(requestMessage)

	nowStr := time.Now().UTC().Format("20060102T150405")
	err := s.rescheduler.RescheduleAll(nowStr)

	if err != nil {
		s.logger.Printf("reschedule-services error: %s", err)
		s.alerter.Send("reschedule_services", "reschedule", requestMessage, "error", err.Error())
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	message := "Rescheduled all services"
	s.logger.Printf("reschedule-services success: %s", message)
	s.sendAlert("reschedule_services", "reschedule", requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})
}

// RescheduleOneService reschedule one service
func (s *Server) RescheduleOneService(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query()
	service := q.Get("service")

	requestMessage := fmt.Sprintf("Rescheduling service: %s", service)
	s.logger.Print(requestMessage)

	nowStr := time.Now().UTC().Format("20060102T150405")
	err := s.rescheduler.RescheduleService(service, nowStr)

	if err != nil {
		s.logger.Printf("reschedule-service error: %s", err.Error())
		s.alerter.Send("reschedule_service", "reschedule", requestMessage, "error", err.Error())
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	message := fmt.Sprintf("Rescheduled service: %s", service)
	s.logger.Printf("reschedule_service success: %s", message)
	s.sendAlert("reschedule_service", "reschedule", requestMessage, "success", message)
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})

}

func (s *Server) rescheduleServiceWait(isManager bool, targetNodeCnt int, nowStr string) {

	tickerC := make(chan time.Time)
	errC := make(chan error, 1)

	go s.rescheduler.RescheduleServicesWaitForNodes(isManager, targetNodeCnt, nowStr, tickerC, errC)

	for {
		select {
		case t := <-tickerC:
			msg := fmt.Sprintf("scale-nodes-reschedule waiting for %d nodes to come online: %v", targetNodeCnt, t)
			s.logger.Print(msg)
			s.sendAlert("scale_nodes", "reschedule", "", "success", msg)
		case err := <-errC:
			close(tickerC)
			if err != nil {
				s.logger.Printf("scale-nodes-reschedule error: %s", err)
				s.sendAlert("scale_nodes", "reschedule", "", "error", err.Error())
			} else {
				msg := "scale-nodes-reschedule success"
				s.logger.Print(msg)
				s.sendAlert("scale_nodes", "reschedule", "", "success", msg)
			}
			return
		}
	}
}
