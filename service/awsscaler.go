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
	managerConfigName      string `envconfig:"AWS_MANAGER_CONFIG_NAME"`
	workerConfigName       string `envconfig:"AWS_WORKER_CONFIG_NAME"`
	region                 string `envconfig:"AWS_DEFAULT_REGION"`
	defaultMinManagerNodes uint64 `envconfig:"DEFAULT_MIN_MANAGER_NODES"`
	defaultMaxManagerNodes uint64 `envconfig:"DEFAULT_MAX_MANAGER_NODES"`
	defaultMinWorkerNodes  uint64 `envconfig:"DEFAULT_MIN_WORKER_NODES"`
	defaultMaxWorkerNodes  uint64 `envconfig:"DEFAULT_MAX_WORKER_NODES"`
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

	sess, err := session.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create aws session")
	}
	svc := autoscaling.New(sess, aws.NewConfig().WithRegion(spec.region))
	return &AWSScaler{
		svc:  svc,
		spec: spec,
	}, nil
}

// ScaleWorkerByDelta scales aws worker nodes by delta
func (s *AWSScaler) ScaleWorkerByDelta(delta int) (uint64, uint64, error) {
	return s.scaleNodes(delta, s.spec.workerConfigName, int64(s.spec.defaultMinWorkerNodes),
		int64(s.spec.defaultMaxWorkerNodes))
}

// ScaleManagerByDelta scales aws manager nodes by delta
func (s *AWSScaler) ScaleManagerByDelta(delta int) (uint64, uint64, error) {
	return s.scaleNodes(delta, s.spec.managerConfigName, int64(s.spec.defaultMinManagerNodes),
		int64(s.spec.defaultMaxManagerNodes))
}

func (s *AWSScaler) scaleNodes(delta int, configName string, minSize int64, maxSize int64) (uint64, uint64, error) {

	groupsOutput, err := s.svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return 0, 0, errors.Wrap(err, "Unable to describe-auto-scaling-groups")
	}

	var targetGroup *autoscaling.Group
	for _, group := range groupsOutput.AutoScalingGroups {
		if *group.LaunchConfigurationName == configName {
			targetGroup = group
			break
		}
	}
	if targetGroup == nil {
		return 0, 0, fmt.Errorf("Unable to find launch-configuration-name: %s", configName)
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

	_, err = s.svc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{DesiredCapacity: &newCapacity,
		MinSize: &minSize,
		MaxSize: &maxSize})
	if err != nil {
		return 0, 0, errors.Wrap(err,
			fmt.Sprintf("Error calling update-auto-scaling-group with desired-capacity: %d", newCapacity))
	}
	return uint64(currentCapacity), uint64(newCapacity), nil
}
