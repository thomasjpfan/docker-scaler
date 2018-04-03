package cmd

// Reads configuration from environment to create and run scaling service

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/thomasjpfan/docker-scaler/server"
	"github.com/thomasjpfan/docker-scaler/service"
	"github.com/thomasjpfan/docker-scaler/service/cloud"
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

	AwsEnvFile                  string `envconfig:"AWS_ENV_FILE"`
	MinScaleManagerNodeLabel    string `envconfig:"MIN_SCALE_MANAGER_NODE_LABEL"`
	MaxScaleManagerNodeLabel    string `envconfig:"MAX_SCALE_MANAGER_NODE_LABEL"`
	ScaleManagerNodeDownByLabel string `envconfig:"SCALE_MANAGER_NODE_DOWN_BY_LABEL"`
	ScaleManagerNodeUpByLabel   string `envconfig:"SCALE_MANAGER_NODE_UP_BY_LABEL"`
	MinScaleWorkerNodeLabel     string `envconfig:"MIN_SCALE_WORKER_NODE_LABEL"`
	MaxScaleWorkerNodeLabel     string `envconfig:"MAX_SCALE_WORKER_NODE_LABEL"`
	ScaleWorkerNodeDownByLabel  string `envconfig:"SCALE_WORKER_NODE_DOWN_BY_LABEL"`
	ScaleWorkerNodeUpByLabel    string `envconfig:"SCALE_WORKER_NODE_UP_BY_LABEL"`

	DefaultMinManagerNodes uint64 `envconfig:"DEFAULT_MIN_MANAGER_NODES"`
	DefaultMaxManagerNodes uint64 `envconfig:"DEFAULT_MAX_MANAGER_NODES"`
	DefaultMinWorkerNodes  uint64 `envconfig:"DEFAULT_MIN_WORKER_NODES"`
	DefaultMaxWorkerNodes  uint64 `envconfig:"DEFAULT_MAX_WORKER_NODES"`

	DefaultScaleManagerNodeDownBy uint64 `envconfig:"DEFAULT_SCALE_MANAGER_NODE_DOWN_BY"`
	DefaultScaleManagerNodeUpBy   uint64 `envconfig:"DEFAULT_SCALE_MANAGER_NODE_UP_BY"`
	DefaultScaleWorkerNodeDownBy  uint64 `envconfig:"DEFAULT_SCALE_WORKER_NODE_DOWN_BY"`
	DefaultScaleWorkerNodeUpBy    uint64 `envconfig:"DEFAULT_SCALE_WORKER_NODE_UP_BY"`
}

// Run starts docker-scaler service
func Run() {
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
			url, time.Duration(spec.AlertTimeout)*time.Second)
		logger.Printf("Using alertmanager at: %s", url)
	} else {
		alerter = service.NewSilentAlertService()
		logger.Printf("Using a stubbed alertmanager")
	}

	cloudOptions := cloud.NewCloudOptions{
		AWSEnvFile: spec.AwsEnvFile,
	}

	cloud, err := cloud.NewCloud(spec.NodeScalerBackend, cloudOptions)
	if err != nil {
		logger.Printf("No cloud provider for node scaling configured")
	} else {
		logger.Printf("Using node-scaling backend: %s", spec.NodeScalerBackend)
	}

	managerResolveOpts := service.ResolveDeltaOptions{
		MinLabel:           spec.MinScaleManagerNodeLabel,
		MaxLabel:           spec.MaxScaleManagerNodeLabel,
		ScaleDownByLabel:   spec.ScaleManagerNodeDownByLabel,
		ScaleUpByLabel:     spec.ScaleManagerNodeUpByLabel,
		DefaultMin:         spec.DefaultMinManagerNodes,
		DefaultMax:         spec.DefaultMaxManagerNodes,
		DefaultScaleDownBy: spec.DefaultScaleManagerNodeDownBy,
		DefaultScaleUpBy:   spec.DefaultScaleManagerNodeUpBy,
	}
	workerResolveOpts := service.ResolveDeltaOptions{
		MinLabel:           spec.MinScaleWorkerNodeLabel,
		MaxLabel:           spec.MaxScaleWorkerNodeLabel,
		ScaleDownByLabel:   spec.ScaleWorkerNodeDownByLabel,
		ScaleUpByLabel:     spec.ScaleWorkerNodeUpByLabel,
		DefaultMin:         spec.DefaultMinWorkerNodes,
		DefaultMax:         spec.DefaultMaxWorkerNodes,
		DefaultScaleDownBy: spec.DefaultScaleWorkerNodeDownBy,
		DefaultScaleUpBy:   spec.DefaultScaleWorkerNodeUpBy,
	}

	nodeScaler := service.NewNodeScaler(
		cloud, client, managerResolveOpts, workerResolveOpts)

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

	resolveScalerDetlaOpts := service.ResolveDeltaOptions{
		MinLabel:           spec.MinScaleLabel,
		MaxLabel:           spec.MaxScaleLabel,
		ScaleDownByLabel:   spec.ScaleDownByLabel,
		ScaleUpByLabel:     spec.ScaleUpByLabel,
		DefaultMin:         spec.DefaultMinReplicas,
		DefaultMax:         spec.DefaultMaxReplicas,
		DefaultScaleDownBy: spec.DefaultScaleServiceDownBy,
		DefaultScaleUpBy:   spec.DefaultScaleServiceUpBy,
	}

	scalerService := service.NewScalerService(
		client, resolveScalerDetlaOpts)
	s := server.NewServer(scalerService, alerter, nodeScaler,
		rescheduler, logger,
		spec.AlertScaleMin, spec.AlertScaleMax,
		spec.AlertNodeMin, spec.AlertNodeMax)
	s.Run(8080, spec.ServerPrefix)
}
