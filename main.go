// Reads configuration from environemnt to create and run scaling service
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/kelseyhightower/envconfig"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

type specification struct {
	MinScaleLabel       string `envconfig:"MIN_SCALE_LABEL"`
	MaxScaleLabel       string `envconfig:"MAX_SCALE_LABEL"`
	DefaultMinReplicas  uint64 `envconfig:"DEFAULT_MIN_REPLICAS"`
	DefaultMaxReplicas  uint64 `envconfig:"DEFAULT_MAX_REPLICAS"`
	AlertmanagerAddress string `envconfig:"ALERTMANAGER_ADDRESS"`
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
		url := fmt.Sprintf("http://%s:9093", spec.AlertmanagerAddress)
		alerter = service.NewAlertService(url)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.NewSilentAlertService()
		logger.Printf("Using a stubbed alertmanager")
	}

	nodeScalerFactory := service.NewNodeScalerFactory()

	logger.Print("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, spec.MinScaleLabel, spec.MaxScaleLabel,
		spec.DefaultMinReplicas,
		spec.DefaultMaxReplicas)
	s := server.NewServer(scaler, alerter, nodeScalerFactory, logger)
	s.Run(8080)
}
