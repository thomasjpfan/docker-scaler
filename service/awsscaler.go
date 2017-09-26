package service

import (
	"github.com/aws/aws-sdk-go/aws/session"
)

// AWSScaler scales nodes back by Amazon web services
type AWSScaler struct {
	sess *session.Session
}

// NewAWSScaler creates an AWS based node scaler
func NewAWSScaler(envFile string) (*AWSScaler, error) {
	return &AWSScaler{
		sess: nil,
	}, nil
}

// ScaleByDelta scales aws nodes by delta
func (s *AWSScaler) ScaleByDelta(delta int) (uint64, uint64, error) {
	return 0, 0, nil
}
