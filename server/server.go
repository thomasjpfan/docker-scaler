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
	scaler        service.ScalerServicer
	alerter       service.AlertServicer
	nodeScaler    service.NodeScaler
	rescheduler   service.ReschedulerServicer
	logger        *log.Logger
	alertScaleMin bool
	alertScaleMax bool
	alertNodeMin  bool
	alertNodeMax  bool
}

// NewServer creates Server
func NewServer(
	scaler service.ScalerServicer,
	alerter service.AlertServicer,
	nodeScaler service.NodeScaler,
	rescheduler service.ReschedulerServicer,
	logger *log.Logger,
	alertScaleMin bool,
	alertScaleMax bool,
	alertNodeMin bool,
	alertNodeMax bool) *Server {
	return &Server{
		scaler:        scaler,
		alerter:       alerter,
		nodeScaler:    nodeScaler,
		rescheduler:   rescheduler,
		logger:        logger,
		alertScaleMin: alertScaleMin,
		alertScaleMax: alertScaleMax,
		alertNodeMin:  alertNodeMin,
		alertNodeMax:  alertNodeMax,
	}
}

// MakeRouter routes url paths to handlers
func (s *Server) MakeRouter(prefix string) *mux.Router {
	router := mux.NewRouter()
	v1router := router.PathPrefix("/v1").Subrouter()
	s.addRoutes(v1router)
	if prefix != "/" {
		rootPrefix := path.Join(prefix, "/v1")
		v1prefixRouter := router.PathPrefix(rootPrefix).Subrouter()
		s.addRoutes(v1prefixRouter)
	}
	return router
}

func (s *Server) addRoutes(router *mux.Router) {
	router.Path("/scale-service").
		Methods("POST").
		HandlerFunc(s.ScaleService).
		Name("ScaleService")
	router.Path("/scale-nodes").
		Queries("type", "{type}", "by", "{by}").
		Methods("POST").
		HandlerFunc(s.ScaleNodes).
		Name("ScaleNode")
	router.Path("/reschedule-services").
		Methods("POST").
		HandlerFunc(s.RescheduleAllServices).
		Name("RescheduleAllServices")
	router.Path("/reschedule-service").
		Methods("POST").
		Queries("service", "{service}").
		HandlerFunc(s.RescheduleOneService).
		Name("RescheduleOneService")
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
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		message := "Unable to decode POST body"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	var ssReq ScaleRequest
	err = json.Unmarshal(body, &ssReq)

	if err != nil {
		message := "Unable to recognize POST body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	serviceName := ssReq.GroupLabels.Service
	scaleDirection := ssReq.GroupLabels.Scale

	if len(serviceName) == 0 {
		message := "No service name in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if len(scaleDirection) == 0 {
		message := "No scale direction in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction in request body"
		s.logger.Printf("scale-service error: %s, body: %s", message, body)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	requestMessage := fmt.Sprintf("Scale service %s: %s", scaleDirection, serviceName)
	s.logger.Print(requestMessage)

	var message string
	var atBound bool
	if scaleDirection == "down" {
		message, atBound, err = s.scaler.ScaleDown(ctx, serviceName)
	} else {
		message, atBound, err = s.scaler.ScaleUp(ctx, serviceName)
	}

	if err != nil {
		message = err.Error()
		respondWithError(w, http.StatusInternalServerError, message)
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", serviceName, requestMessage, "error", message)
		return
	}

	s.logger.Printf("scale-service success: %s", message)
	if !atBound ||
		(scaleDirection == "up" && s.alertScaleMax) ||
		(scaleDirection == "down" && s.alertScaleMin) {
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
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	byInt, err := strconv.Atoi(byStr)

	if err != nil {
		message := fmt.Sprintf("Non integer by query parameter: %s", byStr)
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		message := "Unable to decode POST body"
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	var ssReq ScaleRequest
	err = json.Unmarshal(body, &ssReq)

	if err != nil {
		message := "Unable to recognize POST body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	scaleDirection := ssReq.GroupLabels.Scale

	if len(scaleDirection) == 0 {
		message := "No scale direction in request body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction in request body"
		s.logger.Printf("scale-nodes error: %s, body: %s", message, body)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
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

	var message string
	if scaleDirection == "up" && nodesBefore == nodesNow {
		message = fmt.Sprintf("%s nodes are already scaled to the maximum number of %d nodes", typeStr, nodesNow)
	} else if scaleDirection == "down" && nodesBefore == nodesNow {
		message = fmt.Sprintf("%s nodes are already descaled to the minimum number of %d nodes", typeStr, nodesNow)
	} else {
		message = fmt.Sprintf("Changing the number of %s nodes on %s from %d to %d", typeStr, s.nodeScaler, nodesBefore, nodesNow)
	}

	s.logger.Printf("scale-nodes success: %s", message)

	if nodesBefore != nodesNow ||
		(scaleDirection == "up" && s.alertNodeMax) ||
		(scaleDirection == "down" && s.alertNodeMin) {
		s.sendAlert("scale_nodes", fmt.Sprint(s.nodeScaler), requestMessage, "success", message)
	}
	respondWithJSON(w, http.StatusOK, Response{Status: "OK", Message: message})

	// Call rescheduler if nodesNow is greater than nodesBefore

	if nodesNow > nodesBefore {
		rightNow := time.Now().UTC().Format("20060102T150405")
		reqMsg := fmt.Sprintf("Waiting for %s nodes to scale from %d to %d for rescheduling", typeStr, nodesBefore, nodesNow)
		s.logger.Printf("scale-nodes: %s", reqMsg)
		s.sendAlert("scale_nodes", "reschedule", "Wait to reschedule", "success", reqMsg)

		go s.rescheduleServiceWait(isManager, typeStr, int(nodesBefore), int(nodesNow), rightNow)
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

func (s *Server) rescheduleServiceWait(isManager bool, typeStr string, previousNodeCnt int, targetNodeCnt int, nowStr string) {

	tickerC := make(chan time.Time)
	errC := make(chan error, 1)

	go s.rescheduler.RescheduleServicesWaitForNodes(isManager, targetNodeCnt, nowStr, tickerC, errC)

	requestMsg := "Waiting for nodes to scale up"
	deltaCnt := targetNodeCnt - previousNodeCnt

	timeStart := time.Now().UTC()

	for {
		select {
		case t := <-tickerC:
			msg := fmt.Sprintf("Waiting %f seconds for %d %s nodes to come online", t.Sub(timeStart).Seconds(), deltaCnt, typeStr)
			s.logger.Printf("scale-nodes-reschedule: %s", msg)
			s.sendAlert("reschedule_service", "reschedule", requestMsg, "success", msg)
		case err := <-errC:
			close(tickerC)
			if err != nil {
				s.logger.Printf("scale-nodes-reschedule error: %s", err)
				s.sendAlert("reschedule_service", "reschedule", requestMsg, "error", err.Error())
			} else {
				msg := fmt.Sprintf("%d %s nodes are online and services are rescheduled", targetNodeCnt, typeStr)
				s.logger.Print(msg)
				s.sendAlert("reschedule_service", "reschedule", requestMsg, "success", msg)
			}
			return
		}
	}
}
