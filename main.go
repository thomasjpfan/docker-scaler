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
	MinScaleLabel          string `envconfig:"MIN_SCALE_LABEL"`
	MaxScaleLabel          string `envconfig:"MAX_SCALE_LABEL"`
	DefaultMinReplicas     uint64 `envconfig:"DEFAULT_MIN_REPLICAS"`
	DefaultMaxReplicas     uint64 `envconfig:"DEFAULT_MAX_REPLICAS"`
	AlertmanagerAddress    string `envconfig:"ALERTMANAGER_ADDRESS"`
	AwsEnvFile             string `envconfig:"AWS_ENV_FILE"`
	AwsManagerConfigName   string `envconfig:"AWS_MANAGER_CONFIG_NAME"`
	AwsWorkerConfigName    string `envconfig:"AWS_WORKER_CONFIG_NAME"`
	DefaultMinManagerNodes uint64 `envconfig:"DEFAULT_MIN_MANAGER_NODES"`
	DefaultMaxManagerNodes uint64 `envconfig:"DEFAULT_MAX_MANAGER_NODES"`
	DefaultMinWorkerNodes  uint64 `envconfig:"DEFAULT_MIN_WORKER_NODES"`
	DefaultMaxWorkerNodes  uint64 `envconfig:"DEFAULT_MAX_WORKER_NODES"`
}

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)

	var spec specification
	err := envconfig.Process("", &spec)
	if err != nil {
		logger.Panic(err.Error())
	}

	client, _ := client.NewEnvClient()
	defer client.Close()
	_, err = client.Info(context.Background())
	if err != nil {
		logger.Panicln(err)
	}

	var alerter service.AlertServicer
	if len(spec.AlertmanagerAddress) != 0 {
		url := fmt.Sprintf("http://%s:9093", spec.AlertmanagerAddress)
		alerter = service.NewAlertService(url)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.SilentAlertService{}
		logger.Printf("Using a stubbed alertmanager")
	}

	nodeScalerFactory := service.NewNodeScalerFactory()
	nodeScalerFactory.SetAWSOptions(spec.AwsEnvFile)

	logger.Print("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, spec.MinScaleLabel, spec.MaxScaleLabel,
		spec.DefaultMinReplicas,
		spec.DefaultMaxReplicas)
	s := server.NewServer(scaler, alerter, nodeScalerFactory, logger)
	s.Run(8080)
}
