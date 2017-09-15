package service

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// ScalerServicer interface for resizing services
type ScalerServicer interface {
	GetReplicas(serviceName string) (uint64, error)
	SetReplicas(serviceName string, count uint64) error
	GetMinMaxReplicas(serviceName string) (uint64, uint64, error)
}

// ScalerService scales docker services
type ScalerService struct {
	c          *client.Client
	minLabel   string
	maxLabel   string
	defaultMin uint64
	defaultMax uint64
}

// NewScalerService creates a New Docker Swarm Client
func NewScalerService(
	c *client.Client,
	minLabel string,
	maxLabel string,
	defaultMin uint64,
	defaultMax uint64) *ScalerService {
	return &ScalerService{
		c:          c,
		minLabel:   minLabel,
		maxLabel:   maxLabel,
		defaultMin: defaultMin,
		defaultMax: defaultMax,
	}
}

// GetReplicas Gets Replicas
func (s *ScalerService) GetReplicas(serviceName string) (uint64, error) {

	service, _, err := s.c.ServiceInspectWithRaw(context.Background(), serviceName)

	if err != nil {
		return 0, err
	}

	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return currentReplicas, nil
}

// SetReplicas Sets the number of replicas
func (s *ScalerService) SetReplicas(serviceName string, count uint64) error {

	service, _, err := s.c.ServiceInspectWithRaw(context.Background(), serviceName)

	if err != nil {
		return err
	}

	var count2 uint64
	count2 = uint64(count)

	service.Spec.Mode.Replicated.Replicas = &count2
	updateOpts := types.ServiceUpdateOptions{}
	updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec

	_, updateErr := s.c.ServiceUpdate(
		context.Background(), service.ID, service.Version, service.Spec, updateOpts)
	return updateErr
}

// GetMinMaxReplicas gets the min and maximum replicas allowed for serviceName
func (s *ScalerService) GetMinMaxReplicas(serviceName string) (uint64, uint64, error) {

	minReplicas := s.defaultMin
	maxReplicas := s.defaultMax

	service, _, err := s.c.ServiceInspectWithRaw(context.Background(), serviceName)

	if err != nil {
		return minReplicas, maxReplicas, err
	}

	labels := service.Spec.TaskTemplate.ContainerSpec.Labels
	minLabel := labels[s.minLabel]
	maxLabel := labels[s.maxLabel]

	if len(minLabel) > 0 {
		minReplicasLabel, err := strconv.Atoi(minLabel)
		if err == nil {
			minReplicas = uint64(minReplicasLabel)
		}
	}
	if len(maxLabel) > 0 {
		maxReplicasLabel, err := strconv.Atoi(maxLabel)
		if err == nil {
			maxReplicas = uint64(maxReplicasLabel)
		}
	}

	return minReplicas, maxReplicas, err
}
