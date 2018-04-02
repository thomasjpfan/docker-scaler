package cloud

import "context"

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
