package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

type silentNodeScaler struct{}

func (s silentNodeScaler) ScaleManagerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	return 0, 0, fmt.Errorf("node-scaler not configured with a backend")
}

func (s silentNodeScaler) ScaleWorkerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	return 0, 0, fmt.Errorf("node-scaler not configured with a backend")
}

// NewNodeScaler creates a node scaler
func NewNodeScaler(nodeBackend string) (NodeScaler, error) {
	switch nodeBackend {
	case "aws":
		scaler, err := NewAWSScalerFromEnv()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to create aws scaler")
		}
		return scaler, nil
	default:
		scaler := silentNodeScaler{}
		return &scaler, nil
	}
}
