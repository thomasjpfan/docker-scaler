package service

import "context"

// NodeScaler is an interface for node scaling
type NodeScaler interface {
	// ScaleManagerByDelta returns the number of old worker nodes and new worker nodes
	ScaleManagerByDelta(ctx context.Context, delta int) (uint64, uint64, error)
	// ScaleWorkerByDelta returns the number of old manager nodes and new manager nodes
	ScaleWorkerByDelta(ctx context.Context, delta int) (uint64, uint64, error)
}
