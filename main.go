// Reads configuration from environemnt to create and run scaling service
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
)

func main() {

	logger := log.New(os.Stdout, "", log.LstdFlags)
	client, err := client.NewEnvClient()
	minLabel := "com.df.scaleMin"
	maxLabel := "com.df.scaleMax"

	if err != nil {
		logger.Panicln(err)
	}

	fmt.Println("Starting Docker Scaler")
	scaler := service.NewScalerService(client, minLabel, maxLabel)
	s := server.NewServer(scaler, logger)
	s.Run(8080)
}
