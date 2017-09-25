// Reads configuration from environemnt to create and run scaling service
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/docker/docker/client"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)
	minLabel := os.Getenv("MIN_SCALE_LABEL")
	maxLabel := os.Getenv("MAX_SCALE_LABEL")
	defaultMinReplicasStr := os.Getenv("DEFAULT_MIN_REPLICAS")
	defaultMaxReplicasStr := os.Getenv("DEFAULT_MAX_REPLICAS")
	alertmangerAddress := os.Getenv("ALERTMANAGER_ADDRESS")

	// Check defaultReplicas
	defaultMinReplicasInt, err := strconv.Atoi(defaultMinReplicasStr)
	if err != nil {
		logger.Panic("DEFAULT_MIN_REPLICAS is not an integer")
	}
	defaultMaxReplicasInt, err := strconv.Atoi(defaultMaxReplicasStr)
	if err != nil {
		logger.Panic("DEFAULT_MAX_REPLICAS is not an integer")
	}
	if defaultMinReplicasInt <= 0 {
		logger.Panic("DEFAULT_MIN_REPLICAS must be at least one")
	}
	if defaultMaxReplicasInt <= 0 {
		logger.Panic("DEFAULT_MAX_REPLICAS must be at least one")
	}
	defaultMinReplicas := uint64(defaultMinReplicasInt)
	defaultMaxReplicas := uint64(defaultMaxReplicasInt)

	client, _ := client.NewEnvClient()
	defer client.Close()
	_, err = client.Info(context.Background())
	if err != nil {
		logger.Panicln(err)
	}

	var alerter service.AlertServicer
	if len(alertmangerAddress) != 0 {
		url := fmt.Sprintf("http://%s:9093", alertmangerAddress)
		alerter = service.NewAlertService(url)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.SilentAlertService{}
		logger.Printf("Using a stubbed alertmanager")
	}

	logger.Print("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, minLabel, maxLabel,
		defaultMinReplicas,
		defaultMaxReplicas)
	s := server.NewServer(scaler, alerter, logger)
	s.Run(8080)
}
