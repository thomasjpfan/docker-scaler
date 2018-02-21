package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types"
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

	_, maxReplicas, err := s.getMinMaxReplicas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

	currentReplicas, err := s.getReplicas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

	_, scaleUpBy, err := s.getDownUpScaleDeltas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

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

	err = s.setReplicas(ctx, serviceName, newReplicas)
	if err != nil {
		return "", false, err
	}

	message := fmt.Sprintf("Scaling %s from %d to %d replicas (max: %d)", serviceName, currentReplicas, newReplicas, maxReplicas)
	return message, false, nil
}

func (s *scalerService) ScaleDown(ctx context.Context, serviceName string) (string, bool, error) {
	minReplicas, _, err := s.getMinMaxReplicas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

	currentReplicas, err := s.getReplicas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

	scaleDownBy, _, err := s.getDownUpScaleDeltas(ctx, serviceName)
	if err != nil {
		return "", false, err
	}

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

	err = s.setReplicas(ctx, serviceName, newReplicas)
	if err != nil {
		return "", false, err
	}

	message := fmt.Sprintf("Scaling %s from %d to %d replicas (min: %d)", serviceName, currentReplicas, newReplicas, minReplicas)
	return message, false, nil
}

// getReplicas Gets Replicas
func (s *scalerService) getReplicas(ctx context.Context, serviceName string) (uint64, error) {

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

	if err != nil {
		return 0, errors.Wrap(err, "docker inspect failed in ScalerService")
	}

	currentReplicas := *service.Spec.Mode.Replicated.Replicas
	return currentReplicas, nil
}

// setReplicas Sets the number of replicas
func (s *scalerService) setReplicas(ctx context.Context, serviceName string, count uint64) error {

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

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

// getMinMaxReplicas gets the min and maximum replicas allowed for serviceName
func (s *scalerService) getMinMaxReplicas(ctx context.Context, serviceName string) (uint64, uint64, error) {

	minReplicas := s.defaultMin
	maxReplicas := s.defaultMax

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

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

	return minReplicas, maxReplicas, nil
}

// getDownUpScaleDeltas gets how much to scale service up or down by
func (s *scalerService) getDownUpScaleDeltas(ctx context.Context, serviceName string) (uint64, uint64, error) {
	scaleDownBy := s.defaultScaleDownBy
	scaleUpBy := s.defaultScaleUpBy

	service, _, err := s.c.ServiceInspectWithRaw(
		ctx, serviceName, types.ServiceInspectOptions{})

	if err != nil {
		return scaleDownBy, scaleUpBy, errors.Wrap(err, "docker inspect failed in ScalerService")
	}

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

	return scaleDownBy, scaleUpBy, nil
}
