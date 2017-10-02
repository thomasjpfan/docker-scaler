package service

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

// AWSScaler scales nodes back by Amazon web services
type AWSScaler struct {
	svc  *autoscaling.AutoScaling
	spec AWSSpec
}

// AWSSpec defaults the specification for aws node scaling
type AWSSpec struct {
	ManagerGroupName       string `envconfig:"AWS_MANAGER_GROUP_NAME"`
	WorkerGroupName        string `envconfig:"AWS_WORKER_GROUP_NAME"`
	DefaultMinManagerNodes uint64 `envconfig:"DEFAULT_MIN_MANAGER_NODES"`
	DefaultMaxManagerNodes uint64 `envconfig:"DEFAULT_MAX_MANAGER_NODES"`
	DefaultMinWorkerNodes  uint64 `envconfig:"DEFAULT_MIN_WORKER_NODES"`
	DefaultMaxWorkerNodes  uint64 `envconfig:"DEFAULT_MAX_WORKER_NODES"`
}

// NewAWSScalerFromEnv creates an AWS based node scaler
func NewAWSScalerFromEnv() (*AWSScaler, error) {

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

// ScaleWorkerByDelta scales aws worker nodes by delta
func (s *AWSScaler) ScaleWorkerByDelta(delta int) (uint64, uint64, error) {
	return s.scaleNodes(delta, s.spec.WorkerGroupName, int64(s.spec.DefaultMinWorkerNodes),
		int64(s.spec.DefaultMaxWorkerNodes))
}

// ScaleManagerByDelta scales aws manager nodes by delta
func (s *AWSScaler) ScaleManagerByDelta(delta int) (uint64, uint64, error) {
	return s.scaleNodes(delta, s.spec.ManagerGroupName, int64(s.spec.DefaultMinManagerNodes),
		int64(s.spec.DefaultMaxManagerNodes))
}

func (s *AWSScaler) scaleNodes(delta int, groupName string, minSize int64, maxSize int64) (uint64, uint64, error) {

	groupsOutput, err := s.svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return 0, 0, errors.Wrap(err, "Unable to describe-auto-scaling-groups")
	}

	var targetGroup *autoscaling.Group
	for _, group := range groupsOutput.AutoScalingGroups {
		if *group.AutoScalingGroupName == groupName {
			targetGroup = group
			break
		}
	}
	if targetGroup == nil {
		return 0, 0, fmt.Errorf("Unable to find launch-configuration-name: %s", groupName)
	}

	currentCapacity := *targetGroup.DesiredCapacity
	newCapacity := currentCapacity + int64(delta)
	if newCapacity < 0 {
		newCapacity = 0
	}

	// Check if newCapacity is in bounds
	if newCapacity < minSize || newCapacity > maxSize {
		return 0, 0, fmt.Errorf("New capacity: %d is not in between %d and %d", newCapacity, minSize, maxSize)
	}

	_, err = s.svc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
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
