package cloud

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AWSScalerTestSuite struct {
	suite.Suite
	awsAutoScalerMock *AWSAutoScalerMock
	awsScaler         *AWSScaler
}

func TestAWSScalerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(AWSScalerTestSuite))
}

func (s *AWSScalerTestSuite) SetupTest() {
	s.awsAutoScalerMock = new(AWSAutoScalerMock)
	spec := AWSSpec{
		ManagerASG: "managerASG",
		WorkerASG:  "workerASG",
	}
	s.awsScaler = &AWSScaler{
		svc:  s.awsAutoScalerMock,
		spec: spec,
	}
}

func (s *AWSScalerTestSuite) Test_String() {
	s.Equal("aws", s.awsScaler.String())
}

func (s *AWSScalerTestSuite) Test_NewAWSScalerFromENV_UndefinedAWS_MANAGER_ASG_ReturnsError() {
	_, err := NewAWSScalerFromEnv()
	s.Error(err)
	s.Equal("AWS Scaling requires AWS_MANAGER_ASG", err.Error())
}

func (s *AWSScalerTestSuite) Test_NewAWSScalerFromENV_UndefinedAWS_WORKER_ASG_ReturnsError() {
	defer func() {
		os.Unsetenv("AWS_MANAGER_ASG")
	}()
	os.Setenv("AWS_MANAGER_ASG", "awsmanager")
	_, err := NewAWSScalerFromEnv()
	s.Error(err)
	s.Equal("AWS Scaling requires AWS_WORKER_ASG", err.Error())
}

func (s *AWSScalerTestSuite) Test_NewAWSScalerFromENV() {
	defer func() {
		os.Unsetenv("AWS_MANAGER_ASG")
		os.Unsetenv("AWS_WORKER_ASG")
	}()
	os.Setenv("AWS_MANAGER_ASG", "awsmanager")
	os.Setenv("AWS_WORKER_ASG", "awsworker")
	_, err := NewAWSScalerFromEnv()
	s.NoError(err)
}

func (s *AWSScalerTestSuite) Test_GetNodes_DescribeAutoScalingErrors_ReturnsError() {
	retErr := errors.New("Error")
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{}
	s.awsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, retErr)

	_, err := s.awsScaler.GetNodes(context.Background(), NodeManagerType)
	s.Require().Error(err)

	s.Equal("Unable to describe-auto-scaling-groups: Error", err.Error())
	s.awsAutoScalerMock.AssertExpectations(s.T())

}

func (s *AWSScalerTestSuite) Test_GetNodes_NoValidTargetGroup_ReturnsError() {
	wrongGroupName := "wrongName"
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &wrongGroupName,
			},
		},
	}
	s.awsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	_, err := s.awsScaler.GetNodes(context.Background(), NodeManagerType)
	s.Require().Error(err)

	s.Equal("Unable to find launch-configuration-name: managerASG", err.Error())
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_GetNodes_NoValidDesiredCapacity_ReturnsError() {
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.awsScaler.spec.WorkerASG,
			},
		},
	}
	s.awsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	_, err := s.awsScaler.GetNodes(context.Background(), NodeWorkerType)
	s.Require().Error(err)
	s.Equal("DesiredCapacity not set on aws", err.Error())
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_GetNodes_NegativeDesiredCapacity_ReturnsError() {
	expCurrentCap := int64(-4)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.awsScaler.spec.ManagerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	s.awsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	_, err := s.awsScaler.GetNodes(context.Background(), NodeManagerType)
	s.Require().Error(err)
	s.Equal("Current capacity is less than zero", err.Error())
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_GetNodes() {
	expCurrentCap := int64(4)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.awsScaler.spec.WorkerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	s.awsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	nodes, err := s.awsScaler.GetNodes(context.Background(), NodeWorkerType)
	s.Require().NoError(err)
	s.Equal(uint64(4), nodes)
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_SetNodes_UpdateAutoScalingError() {
	expNodes := int64(4)
	minSize := int64(1)
	maxSize := int64(10)
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.awsScaler.spec.ManagerASG,
		DesiredCapacity:      &expNodes,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	autoScaleError := errors.New("Auto scaling error")
	s.awsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, autoScaleError)

	err := s.awsScaler.SetNodes(context.Background(), NodeManagerType, 4, 1, 10)
	s.Require().Error(err)
	s.Equal("Error calling update-auto-scaling-group with desired-capacity: 4: Auto scaling error", err.Error())
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_SetNodes_Manager() {

	expNodes := int64(4)
	minSize := int64(1)
	maxSize := int64(10)
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.awsScaler.spec.ManagerASG,
		DesiredCapacity:      &expNodes,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.awsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	err := s.awsScaler.SetNodes(context.Background(), NodeManagerType, 4, 1, 10)
	s.Require().NoError(err)
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AWSScalerTestSuite) Test_SetNodes_Worker() {
	expNodes := int64(4)
	minSize := int64(1)
	maxSize := int64(10)
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.awsScaler.spec.WorkerASG,
		DesiredCapacity:      &expNodes,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.awsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	err := s.awsScaler.SetNodes(context.Background(), NodeWorkerType, 4, 1, 10)
	s.Require().NoError(err)
	s.awsAutoScalerMock.AssertExpectations(s.T())
}

type AWSAutoScalerMock struct {
	mock.Mock
}

func (m *AWSAutoScalerMock) DescribeAutoScalingGroupsWithContext(ctx aws.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...request.Option) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), args.Error(1)
}

func (m *AWSAutoScalerMock) UpdateAutoScalingGroupWithContext(ctx aws.Context, input *autoscaling.UpdateAutoScalingGroupInput, opts ...request.Option) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.UpdateAutoScalingGroupOutput), args.Error(1)
}
