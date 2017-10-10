package service

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// ScalerServicer interface for resizing services
type ScalerServicer interface {
	GetReplicas(ctx context.Context, serviceName string) (uint64, error)
	SetReplicas(ctx context.Context, serviceName string, count uint64) error
	GetMinMaxReplicas(ctx context.Context, serviceName string) (uint64, uint64, error)
}

// ScalerService scales docker services
type scalerService struct {
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
	defaultMax uint64) ScalerServicer {
	return &scalerService{
		c:          c,
		minLabel:   minLabel,
		maxLabel:   maxLabel,
		defaultMin: defaultMin,
		defaultMax: defaultMax,
	}
}

// GetReplicas Gets Replicas
func (s *scalerService) GetReplicas(ctx context.Context, serviceName string) (uint64, error) {

	service, _, err := s.c.ServiceInspectWithRaw(ctx, serviceName)

	if err != nil {
		return 0, errors.Wrap(err, "docker inspect failed in ScalerService")
	}

	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return currentReplicas, nil
}

// SetReplicas Sets the number of replicas
func (s *scalerService) SetReplicas(ctx context.Context, serviceName string, count uint64) error {

	service, _, err := s.c.ServiceInspectWithRaw(ctx, serviceName)

	if err != nil {
		return errors.Wrap(err, "docker inspect failed in ScalerService")
	}

	service.Spec.Mode.Replicated.Replicas = &count
	updateOpts := types.ServiceUpdateOptions{}
	updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec

	_, updateErr := s.c.ServiceUpdate(
		ctx, service.ID, service.Version, service.Spec, updateOpts)
	return updateErr
}

// GetMinMaxReplicas gets the min and maximum replicas allowed for serviceName
func (s *scalerService) GetMinMaxReplicas(ctx context.Context, serviceName string) (uint64, uint64, error) {

	minReplicas := s.defaultMin
	maxReplicas := s.defaultMax

	service, _, err := s.c.ServiceInspectWithRaw(ctx, serviceName)

	if err != nil {
		return minReplicas, maxReplicas, errors.Wrap(err, "docker inspect failed in ScalerService")
	}

	labels := service.Spec.Labels
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
