package service

// NodeScaler is an interface for node scaling
type NodeScaler interface {
	ScaleByDelta(delta int) error
}
