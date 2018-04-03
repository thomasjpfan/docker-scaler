package cloud

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

// NodeType is either manager or worker
type NodeType string

const (
	// NodeManagerType is the mananger type
	NodeManagerType NodeType = "manager"
	// NodeWorkerType is the worker type
	NodeWorkerType NodeType = "worker"
)

// Cloud is an interface for cloud providers
type Cloud interface {
	GetNodes(ctx context.Context, nodeType NodeType) (uint64, error)
	SetNodes(ctx context.Context, nodeType NodeType, cnt, minSize, maxSize uint64) error
	String() string
}

// NewCloudOptions are options for creating cloud objects
type NewCloudOptions struct {
	// AWS
	AWSEnvFile string
}

// NewCloud creates a new Cloud object
func NewCloud(nodeBackend string, opts NewCloudOptions) (Cloud, error) {
	switch nodeBackend {
	case "aws":
		if len(opts.AWSEnvFile) > 0 {
			err := godotenv.Load(opts.AWSEnvFile)
			if err != nil {
				return nil, errors.Wrapf(err, "Unable to load %s", opts.AWSEnvFile)
			}
		}
		c, err := NewAWSScalerFromEnv()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to create aws cloud")
		}
		return c, nil
	}
	return nil, fmt.Errorf("backend %s does not exist", nodeBackend)
}
