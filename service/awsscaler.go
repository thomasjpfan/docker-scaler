package service

import (
	"github.com/aws/aws-sdk-go/aws/session"
)

// AWSScaler scales nodes back by Amazon web services
type AWSScaler struct {
	sess *session.Session
}

func NewAWSScaler(envFile string) (*AWSScaler, error) {
	return &AWSScaler{
		sess: nil,
	}, nil
}

// ScaleDelta scales aws nodes by delta
func (s *AWSScaler) ScaleByDelta(delta int) error {
	return nil
}
