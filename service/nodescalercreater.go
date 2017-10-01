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
type NodeScalerFactory struct{}

// NewNodeScalerFactory creates a factory for generating NodeScalers
func NewNodeScalerFactory() *NodeScalerFactory {
	return &NodeScalerFactory{}
}

// New creates NodeScalers with a passed in node backend
func (f *NodeScalerFactory) New(nodeBackend string) (NodeScaler, error) {
	switch nodeBackend {
	case "aws":
		scaler, err := NewAWSScalerFromEnv()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to create aws scaler")
		}
		return scaler, nil
	}
	return nil, fmt.Errorf("Node Backend %s is not supported", nodeBackend)
}
