// +build !test

package service

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

type awsAutoScaler interface {
	DescribeAutoScalingGroupsWithContext(ctx aws.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...request.Option) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	UpdateAutoScalingGroupWithContext(ctx aws.Context, input *autoscaling.UpdateAutoScalingGroupInput, opts ...request.Option) (*autoscaling.UpdateAutoScalingGroupOutput, error)
}

type awsScaler struct {
	svc  awsAutoScaler
	spec AWSSpec
}

// AWSSpec defaults the specification for aws node scaling
type AWSSpec struct {
	ManagerASG             string `envconfig:"AWS_MANAGER_ASG"`
	WorkerASG              string `envconfig:"AWS_WORKER_ASG"`
	DefaultMinManagerNodes uint64 `envconfig:"DEFAULT_MIN_MANAGER_NODES"`
	DefaultMaxManagerNodes uint64 `envconfig:"DEFAULT_MAX_MANAGER_NODES"`
	DefaultMinWorkerNodes  uint64 `envconfig:"DEFAULT_MIN_WORKER_NODES"`
	DefaultMaxWorkerNodes  uint64 `envconfig:"DEFAULT_MAX_WORKER_NODES"`
}

// NewAWSScalerFromEnv creates an AWS based node scaler
func NewAWSScalerFromEnv() (NodeScaler, error) {

	envFile := os.Getenv("AWS_ENV_FILE")
	if len(envFile) == 0 {
		return nil, fmt.Errorf("AWS_ENV_FILE not defined")
	}

	godotenv.Load(envFile)

	var spec AWSSpec
	err := envconfig.Process("", &spec)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get process env vars")
	}

	if len(spec.ManagerASG) == 0 {
		return nil, fmt.Errorf("AWS Scaling requires AWS_MANAGER_ASG")
	}

	if len(spec.WorkerASG) == 0 {
		return nil, fmt.Errorf("AWS Scaling requires AWS_WORKER_ASG")
	}

	sess, err := session.NewSessionWithOptions(
		session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create aws session")
	}

	svc := autoscaling.New(sess, aws.NewConfig())
	return &awsScaler{
		svc:  svc,
		spec: spec,
	}, nil
}

// ScaleWorkerByDelta scales aws worker nodes by delta
func (s *awsScaler) ScaleWorkerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	return s.scaleNodes(ctx, int64(delta), s.spec.WorkerASG, int64(s.spec.DefaultMinWorkerNodes),
		int64(s.spec.DefaultMaxWorkerNodes))
}

// ScaleManagerByDelta scales aws manager nodes by delta
func (s *awsScaler) ScaleManagerByDelta(ctx context.Context, delta int) (uint64, uint64, error) {
	return s.scaleNodes(ctx, int64(delta), s.spec.ManagerASG, int64(s.spec.DefaultMinManagerNodes),
		int64(s.spec.DefaultMaxManagerNodes))
}

// String conforms to Stringer interface
func (s *awsScaler) String() string {
	return "aws"
}

func (s *awsScaler) scaleNodes(ctx context.Context, delta int64, groupName string, minSize int64, maxSize int64) (uint64, uint64, error) {

	groupsOutput, err := s.svc.DescribeAutoScalingGroupsWithContext(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return 0, 0, errors.Wrap(err, "Unable to describe-auto-scaling-groups")
	}

	var targetGroup *autoscaling.Group
	for _, group := range groupsOutput.AutoScalingGroups {
		if group.AutoScalingGroupName != nil && *group.AutoScalingGroupName == groupName {
			targetGroup = group
			break
		}
	}
	if targetGroup == nil {
		return 0, 0, fmt.Errorf("Unable to find launch-configuration-name: %s", groupName)
	}

	if targetGroup.DesiredCapacity == nil {
		return 0, 0, fmt.Errorf("DesiredCapacity not set on aws")
	}

	currentCapacity := *targetGroup.DesiredCapacity
	newCapacity := currentCapacity + delta
	if newCapacity < minSize {
		newCapacity = minSize
	} else if newCapacity > maxSize {
		newCapacity = maxSize
	}

	if newCapacity == currentCapacity {
		return uint64(currentCapacity), uint64(newCapacity), nil
	}

	_, err = s.svc.UpdateAutoScalingGroupWithContext(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &groupName,
		DesiredCapacity:      &newCapacity,
		MinSize:              &minSize,
		MaxSize:              &maxSize})
	if err != nil {
		return 0, 0, errors.Wrap(err,
			fmt.Sprintf("Error calling update-auto-scaling-group with desired-capacity: %d", newCapacity))
	}
	return uint64(currentCapacity), uint64(newCapacity), nil
}
