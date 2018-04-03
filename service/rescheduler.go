package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
)

// ReschedulerServicer is an interface for rescheduling services
type ReschedulerServicer interface {
	RescheduleService(serviceID, value string) error
	RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error, statusC chan<- string)
	RescheduleAll(value string) (string, error)
	IsWaitingToReschedule() bool
}

// InfoListUpdaterNodeLister is an interface needd for rescheduling events
type InfoListUpdaterNodeLister interface {
	NodeReadyCnt(ctx context.Context, manager bool) (int, error)
	ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error)
	ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error)
	ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec) error
}

type reschedulerService struct {
	c              InfoListUpdaterNodeLister
	filterLabel    string
	envKey         string
	tickerInterval time.Duration
	timeOut        time.Duration
	cHolder        *cancelHolder
}

type cancelHolder struct {
	funcMap map[string]context.CancelFunc
	mux     sync.RWMutex
}

func newCancelHolder() *cancelHolder {
	return &cancelHolder{
		funcMap: map[string]context.CancelFunc{},
		mux:     sync.RWMutex{},
	}
}

func (h *cancelHolder) CallAndSet(key string, newF context.CancelFunc) {
	h.mux.Lock()
	defer h.mux.Unlock()
	for key, cancel := range h.funcMap {
		if cancel != nil {
			cancel()
		}
		delete(h.funcMap, key)
	}
	h.funcMap[key] = newF
}

func (h *cancelHolder) CallAndDelete(key string) {
	h.mux.Lock()
	defer h.mux.Unlock()
	f, ok := h.funcMap[key]
	if ok && f != nil {
		f()
	}
	delete(h.funcMap, key)
}

func (h *cancelHolder) HasCancel() bool {
	h.mux.Lock()
	defer h.mux.Unlock()
	return len(h.funcMap) > 0
}

// NewReschedulerService creates a reschduler
func NewReschedulerService(
	c InfoListUpdaterNodeLister,
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
		cHolder:        newCancelHolder(),
	}, nil
}

func (r *reschedulerService) RescheduleService(serviceID, value string) error {

	serviceInfo, err := r.c.ServiceInspect(
		context.Background(), serviceID)
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

	err = r.rescheduleSingleService(serviceInfo, value)
	if err != nil {
		return errors.Wrap(err, "Unable to reschedule service")
	}
	return nil
}

func (r *reschedulerService) RescheduleServicesWaitForNodes(manager bool, targetNodeCnt int, value string, tickerC chan<- time.Time, errorC chan<- error, statusC chan<- string) {

	ctx, cancel := context.WithCancel(context.Background())
	r.cHolder.CallAndSet(value, cancel)

	var typeStr string
	if manager {
		typeStr = "manager"
	} else {
		typeStr = "worker"
	}

	go func() {
		defer r.cHolder.CallAndDelete(value)
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

				status, err := r.RescheduleAll(value)
				if err != nil {
					errorC <- err
					return
				}
				statusC <- fmt.Sprintf("%d %s nodes are up, %s", targetNodeCnt, typeStr, status)
				return
			case <-timerChan:
				errorC <- fmt.Errorf("Timeout, waited %f seconds for %d nodes to activate", r.timeOut.Seconds(), targetNodeCnt)
				return
			case <-ctx.Done():
				statusC <- "Rescheduling is canceled by another rescheduler"
				return
			}
		}
	}()

}

func (r *reschedulerService) IsWaitingToReschedule() bool {
	return r.cHolder.HasCancel()
}

func (r *reschedulerService) RescheduleAll(value string) (string, error) {
	labelFitler := filters.NewArgs()
	labelFitler.Add("label", r.filterLabel)

	services, err := r.c.ServiceList(context.Background(), types.ServiceListOptions{Filters: labelFitler})
	if err != nil {
		return "", errors.Wrap(err, "Unable to get service list to reschedule")
	}

	if len(services) == 0 {
		return "No services to reschedule", nil
	}

	failed := make(chan string)
	success := make(chan string)
	done := make(chan struct{})

	failedList := []string{}
	successList := []string{}
	go func() {
		for {
			select {
			case f := <-failed:
				failedList = append(failedList, f)
			case s := <-success:
				successList = append(successList, s)
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for _, service := range services {
		wg.Add(1)
		go func(service swarm.Service) {
			defer wg.Done()
			err = r.rescheduleSingleService(service, value)
			if err != nil {
				failed <- service.Spec.Name
				return
			}
			success <- service.Spec.Name
		}(service)
	}
	wg.Wait()
	done <- struct{}{}

	successStr := strings.Join(successList, ", ")

	if len(failedList) > 0 {
		failedStr := strings.Join(failedList, ", ")
		if len(successStr) > 0 {
			return "", fmt.Errorf("%s failed to reschedule (%s succeeded)", failedStr, successStr)
		}
		return "", fmt.Errorf("%s failed to reschedule", failedStr)
	}
	return fmt.Sprintf("%s rescheduled", successStr), nil
}

func (r *reschedulerService) equalTargetCount(targetNodeCnt int, manager bool) (bool, error) {

	nodeCnt, err := r.c.NodeReadyCnt(context.Background(), manager)
	if err != nil {
		return false, errors.Wrap(err, "Unable to get docker node count")
	}

	return nodeCnt == targetNodeCnt, nil
}

func (r *reschedulerService) rescheduleSingleService(service swarm.Service, value string) error {
	spec := &service.Spec
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{}
	}
	envs := spec.TaskTemplate.ContainerSpec.Env

	addedNewEnv := false
	newEnvs := []string{}
	envAdd := fmt.Sprintf("%s=%s", r.envKey, value)

	for _, env := range envs {

		// Already exist in env
		if env == envAdd {
			return nil
		}

		envSplit := strings.SplitN(env, "=", 2)
		if len(envSplit) <= 1 {
			newEnvs = append(newEnvs, env)
			continue
		}

		if envSplit[0] == r.envKey && envSplit[1] != value {
			// env variable updated
			addedNewEnv = true
			newEnvs = append(newEnvs, envAdd)
		} else {
			newEnvs = append(newEnvs, env)
		}
	}

	// envAdd is not in service environment
	// add to newEnvs
	if !addedNewEnv {
		newEnvs = append(newEnvs, envAdd)
	}

	spec.TaskTemplate.ContainerSpec.Env = newEnvs

	err := r.c.ServiceUpdate(context.Background(), service.ID, service.Version, *spec)
	if err != nil {
		return err
	}

	return nil
}
