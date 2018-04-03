package service

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
)

// ScalerServicer interface for resizing services
type ScalerServicer interface {
	Scale(ctx context.Context, serviceName string, by uint64, direction ScaleDirection) (string, bool, error)
}

// UpdaterInspector is an interface for scaling services
type UpdaterInspector interface {
	ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec) error
	ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error)
}

type scalerService struct {
	c           UpdaterInspector
	resolveOpts ResolveDeltaOptions
}

// NewScalerService creates a New Docker Swarm Client
func NewScalerService(
	c UpdaterInspector,
	resolveOpts ResolveDeltaOptions,
) ScalerServicer {
	return &scalerService{
		c:           c,
		resolveOpts: resolveOpts,
	}
}

func (s scalerService) Scale(ctx context.Context, serviceName string, by uint64, direction ScaleDirection) (string, bool, error) {

	service, err := s.c.ServiceInspect(ctx, serviceName)

	if err != nil {
		return "", false, errors.Wrap(err, "docker inspect failed in ScalerService")
	}

	isGlobal, err := s.isGlobal(service)
	if err != nil {
		return "", false, err
	}
	if isGlobal {
		return "", false, fmt.Errorf(
			"%s is a global service (can not be scaled)", serviceName)
	}
	currentReplicas, err := s.getReplicas(service)
	if err != nil {
		return "", false, err
	}

	minReplicas, maxReplicas, newReplicas := resolveDelta(currentReplicas, by, direction, service.Spec.Labels, s.resolveOpts)

	if currentReplicas == newReplicas {
		message := s.scaledToBoundMessage(serviceName, minReplicas, maxReplicas, newReplicas, direction)
		return message, true, nil
	}

	err = s.setReplicas(ctx, service, newReplicas)
	if err != nil {
		return "", false, err
	}

	message := fmt.Sprintf("Scaling %s from %d to %d replicas (min: %d, max: %d)", serviceName, currentReplicas, newReplicas, minReplicas, maxReplicas)
	return message, false, nil
}

func (s scalerService) scaledToBoundMessage(serviceName string,
	minReplicas, maxReplicas, newReplicas uint64, direction ScaleDirection) string {
	if direction == ScaleDownDirection {
		return fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceName, minReplicas)
	}
	return fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceName, maxReplicas)
}

// getReplicas Gets Replicas
func (s scalerService) getReplicas(service swarm.Service) (uint64, error) {
	if service.Spec.Mode.Replicated.Replicas == nil {
		return 0, fmt.Errorf("%s does not have a replicas value", service.Spec.Name)
	}
	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return currentReplicas, nil
}

// setReplicas Sets the number of replicas
func (s scalerService) setReplicas(ctx context.Context, service swarm.Service, count uint64) error {

	service.Spec.Mode.Replicated.Replicas = &count

	updateErr := s.c.ServiceUpdate(
		ctx, service.ID, service.Version, service.Spec)
	return updateErr
}

func (s scalerService) isGlobal(service swarm.Service) (bool, error) {

	if service.Spec.Mode.Global != nil {
		return true, nil
	}

	if service.Spec.Mode.Replicated != nil {
		return false, nil
	}

	return false, fmt.Errorf("Unable to recognize service model for: %s", service.Spec.Name)
}
