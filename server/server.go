package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/thomasjpfan/docker-scaler/server/handler"
	"github.com/thomasjpfan/docker-scaler/service"
	"github.com/thomasjpfan/docker-scaler/service/cloud"

	"github.com/gorilla/mux"
)

// Server runs service that scales docker services
type Server struct {
	serviceScaler service.ScalerServicer
	alerter       service.AlertServicer
	nodeScaler    service.NodeScaling
	rescheduler   service.ReschedulerServicer
	logger        *log.Logger
	alertScaleMin bool
	alertScaleMax bool
	alertNodeMin  bool
	alertNodeMax  bool
}

// NewServer creates Server
func NewServer(
	serviceScaler service.ScalerServicer,
	alerter service.AlertServicer,
	nodeScaler service.NodeScaling,
	rescheduler service.ReschedulerServicer,
	logger *log.Logger,
	alertScaleMin bool,
	alertScaleMax bool,
	alertNodeMin bool,
	alertNodeMax bool) *Server {
	return &Server{
		serviceScaler: serviceScaler,
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
	router.Path("/ping").
		Methods("GET").
		HandlerFunc(s.PingHandler).
		Name("Ping")
}

// Run starts server
func (s *Server) Run(port uint16, prefix string) {
	address := fmt.Sprintf(":%d", port)
	m := s.MakeRouter(prefix)
	h := handler.RecoveryHandler(s.logger)
	log.Fatal(http.ListenAndServe(address, h(m)))
}

// PingHandler is sends StatusOK (used by healthcheck)
func (s *Server) PingHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ScaleService scales service
func (s *Server) ScaleService(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	var ssReq ScaleRequest

	if r.Body != nil {
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			message := "Unable to recognize POST body"
			s.logger.Printf("scale-service error: %s", message)
			s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
			respondWithError(w, http.StatusBadRequest, message)
			return
		}

		json.Unmarshal(body, &ssReq)
	}

	serviceName, scaleDirection, by, _ := s.getServiceScaleByType(r.URL.Query(), ssReq)

	if len(serviceName) == 0 {
		message := "No service name in request"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if len(scaleDirection) == 0 {
		message := "No scale direction in request"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction in request"
		s.logger.Printf("scale-service error: %s", message)
		s.sendAlert("scale_service", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	requestMessage := fmt.Sprintf("Scale service %s: %s", scaleDirection, serviceName)
	s.logger.Print(requestMessage)

	var message string
	var atBound bool
	var err error
	if scaleDirection == "down" {
		message, atBound, err = s.serviceScaler.Scale(ctx, serviceName, by, service.ScaleDownDirection)
	} else {
		message, atBound, err = s.serviceScaler.Scale(ctx, serviceName, by, service.ScaleUpDirection)
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
	var ssReq ScaleRequest

	if r.Body != nil {
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			message := "Unable to recognize POST body"
			s.logger.Printf("scale-nodes error: %s", message)
			s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
			respondWithError(w, http.StatusBadRequest, message)
			return
		}

		json.Unmarshal(body, &ssReq)
	}

	serviceName, scaleDirection, by, typeStr := s.getServiceScaleByType(r.URL.Query(), ssReq)

	if len(scaleDirection) == 0 {
		message := "No scale direction"
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if scaleDirection != "up" && scaleDirection != "down" {
		message := "Incorrect scale direction"
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", "bad_request", "Incorrect request", "error", message)
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if typeStr != "worker" && typeStr != "manager" {
		message := fmt.Sprintf("Incorrect node type: %s, type can only be worker or manager", typeStr)
		respondWithError(w, http.StatusBadRequest, message)
		s.logger.Printf("scale-nodes error: %s", message)
		s.sendAlert("scale_nodes", fmt.Sprint(s.nodeScaler), "Incorrect request", "error", message)
		return
	}

	requestMessage := fmt.Sprintf("Scale nodes %s on: %s, by: %d, type: %s", scaleDirection, s.nodeScaler, by, typeStr)
	s.logger.Printf(requestMessage)

	isManager := (typeStr == "manager")

	var direction service.ScaleDirection
	var nodeType cloud.NodeType

	if scaleDirection == "up" {
		direction = service.ScaleUpDirection
	} else {
		direction = service.ScaleDownDirection
	}

	if isManager {
		nodeType = cloud.NodeManagerType
	} else {
		nodeType = cloud.NodeWorkerType
	}
	nodesBefore, nodesNow, err := s.nodeScaler.Scale(
		ctx, by, direction, nodeType, serviceName)

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
	}
}

func (s *Server) getServiceScaleByType(q url.Values, ssReq ScaleRequest) (string, string, uint64, string) {

	service := ssReq.GroupLabels.Service
	scale := ssReq.GroupLabels.Scale
	by := ssReq.GroupLabels.By
	typeStr := ssReq.GroupLabels.Type

	if qService := q.Get("service"); len(qService) > 0 {
		service = qService
	}
	if qScale := q.Get("scale"); len(qScale) > 0 {
		scale = qScale
	}
	if byStr := q.Get("by"); len(byStr) > 0 {
		byInt, err := strconv.Atoi(byStr)
		if err == nil {
			if byInt < 0 {
				by = uint64(-1 * byInt)
			} else {
				by = uint64(byInt)
			}
		}
	}

	if qTypeStr := q.Get("type"); len(qTypeStr) > 0 {
		typeStr = qTypeStr
	}

	return service, scale, by, typeStr
}

// RescheduleAllServices reschedules all services
func (s *Server) RescheduleAllServices(w http.ResponseWriter, r *http.Request) {
	requestMessage := "Rescheduling all labeled services"
	s.logger.Print(requestMessage)

	nowStr := time.Now().UTC().Format("20060102T150405")
	message, err := s.rescheduler.RescheduleAll(nowStr)

	if err != nil {
		s.logger.Printf("reschedule-services error: %s", err)
		s.alerter.Send("reschedule_services", "reschedule", requestMessage, "error", err.Error())
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

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
	errC := make(chan error)
	statusC := make(chan string)

	go s.rescheduler.RescheduleServicesWaitForNodes(isManager, targetNodeCnt, nowStr, tickerC, errC, statusC)

	requestMsg := "Waiting for nodes to scale up"
	deltaCnt := targetNodeCnt - previousNodeCnt

	timeStart := time.Now().UTC()

	for {
		select {
		case t := <-tickerC:
			msg := fmt.Sprintf("Waited %d seconds for %d %s nodes to come online", int(t.Sub(timeStart).Seconds()), deltaCnt, typeStr)
			s.logger.Printf("scale-nodes-reschedule: %s", msg)
			s.sendAlert("reschedule_service", "reschedule", requestMsg, "success", msg)
		case err := <-errC:
			if err != nil {
				s.logger.Printf("scale-nodes-reschedule error: %s", err)
				s.sendAlert("reschedule_service", "reschedule", requestMsg, "error", err.Error())
			}
		case status := <-statusC:
			msg := fmt.Sprintf("%d %s nodes are online and %s", targetNodeCnt, status, typeStr)
			s.logger.Printf("scale-nodes-reschedule: %s", msg)
			s.sendAlert("reschedule_service", "reschedule", requestMsg, "success", msg)
			return
		}
	}
}
