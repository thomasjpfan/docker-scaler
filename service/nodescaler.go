package service

// NodeScaler is an interface for node scaling
type NodeScaler interface {
	// ScaleByDelta returns the number of old nodes and new nodes
	ScaleByDelta(delta int) (uint64, uint64, error)
}
