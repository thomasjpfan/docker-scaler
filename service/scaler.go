package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// ScalerServicer interface for resizing services
type ScalerServicer interface {
	ScaleUp(ctx context.Context, serviceName string) (string, bool, error)
	ScaleDown(ctx context.Context, serviceName string) (string, bool, error)
}

type scalerService struct {
	c                  *client.Client
	minLabel           string
	maxLabel           string
	scaleDownByLabel   string
	scaleUpByLabel     string
	defaultMin         uint64
	defaultMax         uint64
	defaultScaleDownBy uint64
	defaultScaleUpBy   uint64
}

// NewScalerService creates a New Docker Swarm Client
func NewScalerService(
	c *client.Client,
	minLabel string,
	maxLabel string,
	scaleDownByLabel string,
	scaleUpByLabel string,
	defaultMin uint64,
	defaultMax uint64,
	defaultScaleDownBy uint64,
	defaultScaleUpBy uint64) ScalerServicer {
	return &scalerService{
		c:                  c,
		minLabel:           minLabel,
		maxLabel:           maxLabel,
		scaleDownByLabel:   scaleDownByLabel,
		scaleUpByLabel:     scaleUpByLabel,
		defaultMin:         defaultMin,
		defaultMax:         defaultMax,
		defaultScaleDownBy: defaultScaleDownBy,
		defaultScaleUpBy:   defaultScaleUpBy,
	}
}

func (s *scalerService) ScaleUp(ctx context.Context, serviceName string) (string, bool, error) {

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

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

	_, maxReplicas := s.getMinMaxReplicas(service)
	currentReplicas := s.getReplicas(service)
	_, scaleUpBy := s.getScaleUpDownDeltas(service)

	newReplicasInt := currentReplicas + scaleUpBy

	var newReplicas uint64
	if newReplicasInt > maxReplicas {
		newReplicas = maxReplicas
	} else {
		newReplicas = newReplicasInt
	}

	if currentReplicas == maxReplicas && newReplicas == maxReplicas {
		message := fmt.Sprintf("%s is already scaled to the maximum number of %d replicas", serviceName, maxReplicas)
		return message, true, nil
	}

	err = s.setReplicas(ctx, service, newReplicas)
	if err != nil {
		return "", false, err
	}

	message := fmt.Sprintf("Scaling %s from %d to %d replicas (max: %d)", serviceName, currentReplicas, newReplicas, maxReplicas)
	return message, false, nil
}

func (s *scalerService) ScaleDown(ctx context.Context, serviceName string) (string, bool, error) {

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

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

	minReplicas, _ := s.getMinMaxReplicas(service)
	currentReplicas := s.getReplicas(service)
	scaleDownBy, _ := s.getScaleUpDownDeltas(service)

	newReplicasInt := int(currentReplicas) - int(scaleDownBy)

	var newReplicas uint64
	if newReplicasInt < int(minReplicas) {
		newReplicas = minReplicas
	} else {
		newReplicas = uint64(newReplicasInt)
	}

	if currentReplicas == minReplicas && newReplicas == minReplicas {
		message := fmt.Sprintf("%s is already descaled to the minimum number of %d replicas", serviceName, minReplicas)
		return message, true, nil
	}

	err = s.setReplicas(ctx, service, newReplicas)
	if err != nil {
		return "", false, err
	}

	message := fmt.Sprintf("Scaling %s from %d to %d replicas (min: %d)", serviceName, currentReplicas, newReplicas, minReplicas)
	return message, false, nil
}

// getReplicas Gets Replicas
func (s *scalerService) getReplicas(service swarm.Service) uint64 {

	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return currentReplicas
}

// setReplicas Sets the number of replicas
func (s *scalerService) setReplicas(ctx context.Context, service swarm.Service, count uint64) error {

	service.Spec.Mode.Replicated.Replicas = &count
	updateOpts := types.ServiceUpdateOptions{}
	updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec

	_, updateErr := s.c.ServiceUpdate(
		ctx, service.ID, service.Version, service.Spec, updateOpts)
	return updateErr
}

// getMinMaxReplicas gets the min and maximum replicas allowed for serviceName
func (s *scalerService) getMinMaxReplicas(service swarm.Service) (uint64, uint64) {

	minReplicas := s.defaultMin
	maxReplicas := s.defaultMax

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

	return minReplicas, maxReplicas
}

// getScaleUpDownDeltas gets how much to scale service up or down by
func (s *scalerService) getScaleUpDownDeltas(service swarm.Service) (uint64, uint64) {
	scaleDownBy := s.defaultScaleDownBy
	scaleUpBy := s.defaultScaleUpBy

	labels := service.Spec.Labels
	downLabel := labels[s.scaleDownByLabel]
	upLabel := labels[s.scaleUpByLabel]

	if len(downLabel) > 0 {
		scaleDownLabel, err := strconv.Atoi(downLabel)
		if err == nil {
			scaleDownBy = uint64(scaleDownLabel)
		}
	}

	if len(upLabel) > 0 {
		scaleUpLabel, err := strconv.Atoi(upLabel)
		if err == nil {
			scaleUpBy = uint64(scaleUpLabel)
		}
	}

	return scaleDownBy, scaleUpBy
}

func (s *scalerService) isGlobal(service swarm.Service) (bool, error) {

	if service.Spec.Mode.Global != nil {
		return true, nil
	}

	if service.Spec.Mode.Replicated != nil {
		return false, nil
	}

	return false, fmt.Errorf("Unable to recognize service model for: %s", service.Spec.Name)
}
