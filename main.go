// +build !test

// Reads configuration from environemnt to create and run scaling service
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

type specification struct {
	ServerPrefix              string `envconfig:"SERVER_PREFIX"`
	MinScaleLabel             string `envconfig:"MIN_SCALE_LABEL"`
	MaxScaleLabel             string `envconfig:"MAX_SCALE_LABEL"`
	AlertScaleMin             bool   `envconfig:"ALERT_SCALE_MIN"`
	AlertScaleMax             bool   `envconfig:"ALERT_SCALE_MAX"`
	DefaultMinReplicas        uint64 `envconfig:"DEFAULT_MIN_REPLICAS"`
	DefaultMaxReplicas        uint64 `envconfig:"DEFAULT_MAX_REPLICAS"`
	ScaleDownByLabel          string `envconfig:"SCALE_DOWN_BY_LABEL"`
	ScaleUpByLabel            string `envconfig:"SCALE_UP_BY_LABEL"`
	DefaultScaleServiceDownBy uint64 `envconfig:"DEFAULT_SCALE_SERVICE_DOWN_BY"`
	DefaultScaleServiceUpBy   uint64 `envconfig:"DEFAULT_SCALE_SERVICE_UP_BY"`
	AlertmanagerAddress       string `envconfig:"ALERTMANAGER_ADDRESS"`
	AlertTimeout              int64  `envconfig:"ALERT_TIMEOUT"`
	RescheduleFilterLabel     string `envconfig:"RESCHEDULE_FILTER_LABEL"`
	RescheduleTickerInterval  int64  `envconfig:"RESCHEDULE_TICKER_INTERVAL"`
	RescheduleTimeOut         int64  `envconfig:"RESCHEDULE_TIMEOUT"`
	RescheduleEnvKey          string `envconfig:"RESCHEDULE_ENV_KEY"`
	NodeScalerBackend         string `envconfig:"NODE_SCALER_BACKEND"`
	AlertNodeMin              bool   `envconfig:"ALERT_NODE_MIN"`
	AlertNodeMax              bool   `envconfig:"ALERT_NODE_MAX"`
}

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)

	var spec specification
	err := envconfig.Process("", &spec)
	if err != nil {
		logger.Panic(err)
	}

	client, err := service.NewDockerClientFromEnv()
	if err != nil {
		logger.Panic(err)
	}
	defer client.Close()

	_, err = client.Info(context.Background())
	if err != nil {
		logger.Panic(err)
	}

	var alerter service.AlertServicer
	if len(spec.AlertmanagerAddress) != 0 {
		url := spec.AlertmanagerAddress
		alerter = service.NewAlertService(
			url,
			time.Duration(spec.AlertTimeout)*time.Second)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.NewSilentAlertService()
		logger.Printf("Using a stubbed alertmanager")
	}

	nodeScaler, err := service.NewNodeScaler(spec.NodeScalerBackend)
	if err != nil {
		logger.Panic(err)
	}
	logger.Printf("Using node-scaling backend: %s", spec.NodeScalerBackend)

	rescheduler, err := service.NewReschedulerService(
		client,
		spec.RescheduleFilterLabel,
		spec.RescheduleEnvKey,
		time.Duration(spec.RescheduleTickerInterval)*time.Second,
		time.Duration(spec.RescheduleTimeOut)*time.Second)

	if err != nil {
		logger.Panic(err)
	}

	logger.Print("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, spec.MinScaleLabel, spec.MaxScaleLabel,
		spec.ScaleDownByLabel, spec.ScaleUpByLabel,
		spec.DefaultMinReplicas,
		spec.DefaultMaxReplicas,
		spec.DefaultScaleServiceDownBy,
		spec.DefaultScaleServiceUpBy)
	s := server.NewServer(scaler, alerter, nodeScaler,
		rescheduler, logger,
		spec.AlertScaleMin, spec.AlertScaleMax,
		spec.AlertNodeMin, spec.AlertNodeMax)
	s.Run(8080, spec.ServerPrefix)
}
