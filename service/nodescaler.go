package service

// NodeScaler is an interface for node scaling
type NodeScaler interface {
	// ScaleManagerByDelta returns the number of old worker nodes and new worker nodes
	ScaleManagerByDelta(delta int) (uint64, uint64, error)
	// ScaleWorkerByDelta returns the number of old manager nodes and new manager nodes
	ScaleWorkerByDelta(delta int) (uint64, uint64, error)
}
