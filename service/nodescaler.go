package service

import (
	"context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	"github.com/thomasjpfan/docker-scaler/service/cloud"
)

// NodeScaling is an interface for node scaling
type NodeScaling interface {
	Scale(ctx context.Context, by uint64, direction ScaleDirection, nodeType cloud.NodeType, serviceName string) (uint64, uint64, error)
	String() string
}

// Inspector is an interface for inspecting services
type Inspector interface {
	ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error)
}

// NodeScaler scales nodes
type NodeScaler struct {
	cloudProvider cloud.Cloud
	inspector     Inspector
	managerOpts   ResolveDeltaOptions
	workerOpts    ResolveDeltaOptions
}

// NewNodeScaler returns new node scaler
func NewNodeScaler(cloudProvider cloud.Cloud,
	inspector Inspector, managerOpts, workerOpts ResolveDeltaOptions) NodeScaling {
	if cloudProvider == nil {
		return nil
	}
	return &NodeScaler{
		cloudProvider: cloudProvider,
		inspector:     inspector,
		managerOpts:   managerOpts,
		workerOpts:    workerOpts,
	}
}

// Scale scales nodes returns
// 1. number of nodes before scaling
// 2. number of nodes after scaling
func (s *NodeScaler) Scale(ctx context.Context, by uint64, direction ScaleDirection, nodeType cloud.NodeType, serviceName string) (uint64, uint64, error) {
	labels := map[string]string{}
	if len(serviceName) > 0 {
		ss, err := s.inspector.ServiceInspect(ctx, serviceName)
		if err != nil {
			return 0, 0, errors.Wrap(err, "node scaling failed")
		}
		labels = ss.Spec.Labels
	}

	currentNodes, err := s.cloudProvider.GetNodes(ctx, nodeType)
	if err != nil {
		return 0, 0, errors.Wrap(err, "node scaling failed")
	}

	var resolveOpts ResolveDeltaOptions
	if nodeType == cloud.NodeManagerType {
		resolveOpts = s.managerOpts
	} else {
		resolveOpts = s.workerOpts
	}

	minBound, maxBound, newNodes := resolveDelta(currentNodes, by, direction, labels, resolveOpts)

	err = s.cloudProvider.SetNodes(ctx, nodeType, newNodes, minBound, maxBound)
	if err != nil {
		return 0, 0, errors.Wrap(err, "node scaling failed")
	}

	return currentNodes, newNodes, nil
}

// String adapts to the String interface
func (s NodeScaler) String() string {
	return s.cloudProvider.String()
}
