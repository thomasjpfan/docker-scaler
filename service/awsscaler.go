package service

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

// AWSScaler scales nodes back by Amazon web services
type AWSScaler struct {
	svc              *autoscaling.AutoScaling
	launchConfigName string
}

// NewAWSScalerFromEnv creates an AWS based node scaler
func NewAWSScalerFromEnv(envFile string) (*AWSScaler, error) {

	godotenv.Load(envFile)
	region := os.Getenv("AWS_DEFAULT_REGION")
	launchConfigName := os.Getenv("AWS_LAUNCH_CONFIGURATION_NAME")

	if len(region) == 0 {
		return nil, fmt.Errorf("AWS_DEFAULT_REGION not defined")
	}
	if len(launchConfigName) == 0 {
		return nil, fmt.Errorf("AWS_LAUNCH_CONFIGURATION_NAME is not defined")
	}

	sess, err := session.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create aws session")
	}
	svc := autoscaling.New(sess, aws.NewConfig().WithRegion(region))
	return &AWSScaler{
		svc:              svc,
		launchConfigName: launchConfigName,
	}, nil
}

// ScaleByDelta scales aws nodes by delta
func (s *AWSScaler) ScaleByDelta(delta int) (uint64, uint64, error) {

	// Get group
	groupsOutput, err := s.svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return 0, 0, errors.Wrap(err, "Unable to describe-auto-scaling-groups")
	}

	var targetGroup *autoscaling.Group
	for _, group := range groupsOutput.AutoScalingGroups {
		if *group.LaunchConfigurationName == s.launchConfigName {
			targetGroup = group
			break
		}
	}
	if targetGroup == nil {
		return 0, 0, fmt.Errorf("Unable to find launch-configuration-name: %s", s.launchConfigName)
	}

	currentCapacity := *targetGroup.DesiredCapacity
	newCapacity := currentCapacity + int64(delta)
	if newCapacity < 0 {
		newCapacity = 0
	}

	_, err = s.svc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{DesiredCapacity: &newCapacity})
	if err != nil {
		return 0, 0, errors.Wrap(err,
			fmt.Sprintf("Error calling update-auto-scaling-group with desired-capacity: %d", newCapacity))
	}
	return uint64(currentCapacity), uint64(newCapacity), nil
}
