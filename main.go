// Reads configuration from environemnt to create and run scaling service
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)
	minLabel := "com.df.scaleMin"
	maxLabel := "com.df.scaleMax"
	defaultMin := uint64(1)
	defaultMax := uint64(20)

	client, _ := client.NewEnvClient()
	defer client.Close()

	_, err := client.Info(context.Background())
	if err != nil {
		logger.Panicln(err)
	}

	fmt.Println("Starting Docker Scaler")
	scaler := service.NewScalerService(
		client, minLabel, maxLabel,
		defaultMin, defaultMax)
	s := server.NewServer(scaler, logger)
	s.Run(8080)
}
