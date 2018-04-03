package cloud

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

type awsAutoScaler interface {
	DescribeAutoScalingGroupsWithContext(ctx aws.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...request.Option) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	UpdateAutoScalingGroupWithContext(ctx aws.Context, input *autoscaling.UpdateAutoScalingGroupInput, opts ...request.Option) (*autoscaling.UpdateAutoScalingGroupOutput, error)
}

// AWSScaler is a scaler for AWS
type AWSScaler struct {
	svc  awsAutoScaler
	spec AWSSpec
}

// AWSSpec are the default specs for aws
type AWSSpec struct {
	ManagerASG string `envconfig:"AWS_MANAGER_ASG"`
	WorkerASG  string `envconfig:"AWS_WORKER_ASG"`
}

// NewAWSScalerFromEnv creats an AWS based node scaler
func NewAWSScalerFromEnv() (*AWSScaler, error) {

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
	return &AWSScaler{
		svc:  svc,
		spec: spec,
	}, nil
}

// GetNodes return the number of nodes
func (s AWSScaler) GetNodes(ctx context.Context, nodeType NodeType) (uint64, error) {
	groupsOutput, err := s.svc.DescribeAutoScalingGroupsWithContext(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return 0, errors.Wrap(err, "Unable to describe-auto-scaling-groups")
	}
	var groupName string
	if nodeType == NodeManagerType {
		groupName = s.spec.ManagerASG
	} else {
		groupName = s.spec.WorkerASG
	}

	var targetGroup *autoscaling.Group
	for _, group := range groupsOutput.AutoScalingGroups {
		if group.AutoScalingGroupName != nil && *group.AutoScalingGroupName == groupName {
			targetGroup = group
			break
		}
	}
	if targetGroup == nil {
		return 0, fmt.Errorf("Unable to find launch-configuration-name: %s", groupName)
	}

	if targetGroup.DesiredCapacity == nil {
		return 0, fmt.Errorf("DesiredCapacity not set on aws")
	}

	currentCapacity := *targetGroup.DesiredCapacity

	if currentCapacity < 0 {
		return 0, fmt.Errorf("Current capacity is less than zero")
	}

	return uint64(currentCapacity), nil

}

// SetNodes sets the number of nodes on AWS
func (s AWSScaler) SetNodes(ctx context.Context, nodeType NodeType, cnt, minSize, maxSize uint64) error {

	var groupName string
	if nodeType == NodeManagerType {
		groupName = s.spec.ManagerASG
	} else {
		groupName = s.spec.WorkerASG
	}
	newCapacity := int64(cnt)
	minSizeInt := int64(minSize)
	maxSizeInt := int64(maxSize)

	_, err := s.svc.UpdateAutoScalingGroupWithContext(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &groupName,
		DesiredCapacity:      &newCapacity,
		MinSize:              &minSizeInt,
		MaxSize:              &maxSizeInt})
	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("Error calling update-auto-scaling-group with desired-capacity: %d", cnt))
	}
	return nil
}

// String in this case AWS
func (s AWSScaler) String() string {
	return "aws"
}
