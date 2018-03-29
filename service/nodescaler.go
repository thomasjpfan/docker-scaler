package service

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

// NodeScaler is an interface for node scaling
type NodeScaler interface {
	// ScaleManagerByDelta returns the number of old worker nodes and new worker nodes
	ScaleManagerByDelta(ctx context.Context, delta int) (uint64, uint64, error)
	// ScaleWorkerByDelta returns the number of old manager nodes and new manager nodes
	ScaleWorkerByDelta(ctx context.Context, delta int) (uint64, uint64, error)
}

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
		envFile := os.Getenv("AWS_ENV_FILE")
		if len(envFile) != 0 {
			err := godotenv.Load(envFile)
			if err != nil {
				return nil, errors.Wrapf(err, "Unable to load %s", envFile)
			}
		}
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
