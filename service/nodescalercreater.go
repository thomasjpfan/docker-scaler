package service

import (
	"fmt"

	"github.com/pkg/errors"
)

// NodeScalerCreater is an interface for a factory that creates NodeScalers
type NodeScalerCreater interface {
	New(nodeBackend string) (NodeScaler, error)
}

// NodeScalerFactory creates NodeScalers
type NodeScalerFactory struct {
	// aws options
	awsEnvFile string
}

// NewNodeScalerFactory creates a factory for generating NodeScalers
func NewNodeScalerFactory() *NodeScalerFactory {
	return &NodeScalerFactory{}
}

// SetAWSOptions Sets options for AWS node scaling
func (f *NodeScalerFactory) SetAWSOptions(awsEnvFile string) {
	f.awsEnvFile = awsEnvFile
}

// New creates NodeScalers with a passed in node backend
func (f *NodeScalerFactory) New(nodeBackend string) (NodeScaler, error) {
	switch nodeBackend {
	case "aws":
		scaler, err := NewAWSScaler(f.awsEnvFile)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to create aws scaler")
		}
		return scaler, nil
	}
	return nil, fmt.Errorf("Node Backend %s is not supported", nodeBackend)
}
