package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

// ReschedulerServicer is an interface for rescheduling services
type ReschedulerServicer interface {
	RescheduleService(serviceID, value string) error
	RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error)
	RescheduleAll(value string) error
}

type reschedulerService struct {
	c              *client.Client
	filterLabel    string
	envKey         string
	tickerInterval time.Duration
	timeOut        time.Duration
}

// NewReschedulerService creates a reschduler
func NewReschedulerService(
	c *client.Client,
	filterLabel string,
	envKey string,
	tickerInterval time.Duration,
	timeOut time.Duration) (ReschedulerServicer, error) {

	if kv := strings.Split(filterLabel, "="); len(kv) != 2 {
		return nil, fmt.Errorf("%s does not have form key=value", filterLabel)
	}

	return &reschedulerService{
		c:              c,
		filterLabel:    filterLabel,
		envKey:         envKey,
		tickerInterval: tickerInterval,
		timeOut:        timeOut,
	}, nil
}

func (r *reschedulerService) getWorkerNodeCount() (int, error) {
	info, err := r.c.Info(context.Background())
	if err != nil {
		return 0, errors.Wrap(err, "Unable to get docker info for node count")
	}
	allNodes := info.Swarm.Nodes
	managerNodes := info.Swarm.Managers
	return allNodes - managerNodes, nil
}

func (r *reschedulerService) getManagerNodeCount() (int, error) {
	info, err := r.c.Info(context.Background())
	if err != nil {
		return 0, errors.Wrap(err, "Unable to get docker info for node count")
	}
	managerNodes := info.Swarm.Managers
	return managerNodes, nil
}

func (r *reschedulerService) RescheduleService(serviceID, value string) error {

	serviceInfo, _, err := r.c.ServiceInspectWithRaw(context.Background(), serviceID)
	if err != nil {
		return errors.Wrapf(err, "Unable to inspect service %s", serviceID)
	}

	kv := strings.Split(r.filterLabel, "=")
	filterValue, ok := serviceInfo.Spec.Labels[kv[0]]

	if !ok {
		return fmt.Errorf("%s is not labeled with %s (no label)", serviceID, r.filterLabel)
	}

	if filterValue != kv[1] {
		return fmt.Errorf("%s is not labeled with %s (%s=%s)", serviceID, r.filterLabel, kv[0], filterValue)
	}

	err = r.reschedulerService(serviceInfo, value)
	if err != nil {
		return errors.Wrap(err, "Unable to reschedule service")
	}
	return nil
}

func (r *reschedulerService) RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error) {

	tickerChan := time.NewTicker(r.tickerInterval).C
	timerChan := time.NewTimer(r.timeOut).C

	for {
		select {
		case tc := <-tickerChan:
			tickerC <- tc
			equalTarget, err := r.equalTargetCount(targetNodeCnt, manager)
			if err != nil {
				errorC <- err
				return
			}
			if !equalTarget {
				continue
			}

			err = r.RescheduleAll(value)
			if err != nil {
				errorC <- err
			}
			errorC <- nil
			return
		case <-timerChan:
			errorC <- fmt.Errorf("Waited %f seconds for %d nodes to activate", r.timeOut.Seconds(), targetNodeCnt)
			return

		}
	}

}

func (r *reschedulerService) RescheduleAll(value string) error {
	labelFitler := filters.NewArgs()
	labelFitler.Add("label", r.filterLabel)

	services, err := r.c.ServiceList(context.Background(), types.ServiceListOptions{Filters: labelFitler})
	if err != nil {
		return errors.Wrap(err, "Unable to get service list to reschedule")
	}

	// Concurrent?
	errorServices := []string{}
	for _, service := range services {
		err = r.reschedulerService(service, value)
		if err != nil {
			errorServices = append(errorServices, service.Spec.Name)
		}
	}
	if len(errorServices) != 0 {
		errorServicesStr := strings.Join(errorServices, ", ")
		return fmt.Errorf("Unable to reschedule services: %s", errorServicesStr)
	}

	return nil
}

func (r *reschedulerService) equalTargetCount(targetNodeCnt int, manager bool) (bool, error) {
	var nodeCnt int
	var err error
	if manager {
		nodeCnt, err = r.getManagerNodeCount()
	} else {
		nodeCnt, err = r.getWorkerNodeCount()
	}

	if err != nil {
		return false, errors.Wrap(err, "Equal target count error")
	}
	return nodeCnt == targetNodeCnt, nil
}

func (r *reschedulerService) reschedulerService(service swarm.Service, value string) error {
	spec := &service.Spec
	envs := spec.TaskTemplate.ContainerSpec.Env

	addedNewEnv := false
	newEnvs := []string{}
	envAdd := fmt.Sprintf("%s=%s", r.envKey, value)

	for _, env := range envs {
		envSplit := strings.Split(env, "=")
		if len(envSplit) != 2 {
			newEnvs = append(newEnvs, env)
			continue
		}

		// Already exist in env
		if env == envAdd {
			return nil
		}

		if envSplit[0] == r.envKey && envSplit[1] != value {
			addedNewEnv = true
			newEnvs = append(newEnvs, envAdd)
		} else {
			newEnvs = append(newEnvs, env)
		}
	}

	if !addedNewEnv {
		newEnvs = append(newEnvs, envAdd)
	}

	spec.TaskTemplate.ContainerSpec.Env = newEnvs
	updateOpts := types.ServiceUpdateOptions{}

	_, err := r.c.ServiceUpdate(context.Background(), service.ID, service.Version, *spec, updateOpts)
	if err != nil {
		return err
	}

	return nil
}
