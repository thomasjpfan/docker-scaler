// Reads configuration from environemnt to create and run scaling service
package main

import (
	"context"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/kelseyhightower/envconfig"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

type specification struct {
	MinScaleLabel             string `envconfig:"MIN_SCALE_LABEL"`
	MaxScaleLabel             string `envconfig:"MAX_SCALE_LABEL"`
	DefaultMinReplicas        uint64 `envconfig:"DEFAULT_MIN_REPLICAS"`
	DefaultMaxReplicas        uint64 `envconfig:"DEFAULT_MAX_REPLICAS"`
	ScaleDownByLabel          string `envconfig:"SCALE_DOWN_BY_LABEL"`
	ScaleUpByLabel            string `envconfig:"SCALE_UP_BY_LABEL"`
	DefaultScaleServiceDownBy uint64 `envconfig:"DEFAULT_SCALE_SERVICE_DOWN_BY"`
	DefaultScaleServiceUpBy   uint64 `envconfig:"DEFAULT_SCALE_SERVICE_UP_BY"`
	AlertmanagerAddress       string `envconfig:"ALERTMANAGER_ADDRESS"`
	NodeScalerBackend         string `envconfig:"NODE_SCALER_BACKEND"`
}

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)

	var spec specification
	err := envconfig.Process("", &spec)
	if err != nil {
		logger.Panic(err)
	}

	client, _ := client.NewEnvClient()
	defer client.Close()
	_, err = client.Info(context.Background())
	if err != nil {
		logger.Panic(err)
	}

	var alerter service.AlertServicer
	if len(spec.AlertmanagerAddress) != 0 {
		url := spec.AlertmanagerAddress
		alerter = service.NewAlertService(url)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.NewSilentAlertService()
		logger.Printf("Using a stubbed alertmanager")
	}

	nodeScaler, err := service.NewNodeScaler(spec.NodeScalerBackend)
	if err != nil {
		logger.Panic(err)
	}
	logger.Printf("Using node-scaling backend: %s", nodeScaler)

	logger.Print("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, spec.MinScaleLabel, spec.MaxScaleLabel,
		spec.ScaleDownByLabel, spec.ScaleUpByLabel,
		spec.DefaultMinReplicas,
		spec.DefaultMaxReplicas,
		spec.DefaultScaleServiceDownBy,
		spec.DefaultScaleServiceUpBy)
	s := server.NewServer(scaler, alerter, nodeScaler, logger)
	s.Run(8080)
}
