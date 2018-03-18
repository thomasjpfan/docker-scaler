package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AwsScalerTestSuite struct {
	suite.Suite
	AWSScaler         *awsScaler
	AwsAutoScalerMock *AwsAutoScalerMock
}

func TestAwsScalerTestSuite(t *testing.T) {
	suite.Run(t, new(AwsScalerTestSuite))
}

func (s *AwsScalerTestSuite) SetupTest() {

	spec := AWSSpec{
		ManagerASG:             "managerASG",
		WorkerASG:              "workerASG",
		DefaultMinManagerNodes: uint64(3),
		DefaultMaxManagerNodes: uint64(5),
		DefaultMinWorkerNodes:  uint64(0),
		DefaultMaxWorkerNodes:  uint64(5),
	}
	s.AwsAutoScalerMock = new(AwsAutoScalerMock)
	s.AWSScaler = &awsScaler{
		svc:  s.AwsAutoScalerMock,
		spec: spec,
	}
}

func (s *AwsScalerTestSuite) Test_ScaleWorkerByDelta_DescribeAutoScalingErrors_ReturnsError() {
	retErr := errors.New("Error")
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, retErr)

	_, _, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), 1)
	s.Error(err)
	s.Contains(err.Error(), "Unable to describe-auto-scaling-groups")
	s.AwsAutoScalerMock.AssertExpectations(s.T())
}

func (s *AwsScalerTestSuite) Test_ScaleWorkerByDelta_NoValidTargetGroup_ReturnsError() {
	wrongGroupName := "wrongName"
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &wrongGroupName,
			},
		},
	}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	_, _, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), 1)
	s.Error(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())
	s.Contains(err.Error(), "Unable to find launch-configuration-name")
}

func (s *AwsScalerTestSuite) Test_ScaleWorkerByDelta_NoValidDesiredCapacity_ReturnsError() {
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
			},
		},
	}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)

	_, _, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), 1)
	s.Error(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())
	s.Contains(err.Error(), "DesiredCapacity not set on aws")

}

func (s *AwsScalerTestSuite) Test_ScaleManagerByDelta_UpToAtMax_ScalesToMax() {
	expCurrentCap := int64(4)
	expNewCap := int64(5)
	minSize := int64(s.AWSScaler.spec.DefaultMinManagerNodes)
	maxSize := int64(s.AWSScaler.spec.DefaultMaxManagerNodes)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.ManagerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.AWSScaler.spec.ManagerASG,
		DesiredCapacity:      &expNewCap,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)
	s.AwsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	currentCap, newCap, err := s.AWSScaler.ScaleManagerByDelta(context.Background(), 2)
	s.Require().NoError(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())

	s.Equal(uint64(expCurrentCap), currentCap)
	s.Equal(uint64(5), newCap)
}

func (s *AwsScalerTestSuite) Test_ScaleManagerByDelta_DownToAtMin_ScalesToMin() {
	expCurrentCap := int64(4)
	expNewCap := int64(3)
	minSize := int64(s.AWSScaler.spec.DefaultMinManagerNodes)
	maxSize := int64(s.AWSScaler.spec.DefaultMaxManagerNodes)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.ManagerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.AWSScaler.spec.ManagerASG,
		DesiredCapacity:      &expNewCap,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)
	s.AwsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	currentCap, newCap, err := s.AWSScaler.ScaleManagerByDelta(context.Background(), -2)
	s.Require().NoError(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())

	s.Equal(uint64(expCurrentCap), currentCap)
	s.Equal(uint64(3), newCap)
}

func (s *AwsScalerTestSuite) Test_ScaleWorkerByDelta_ScalesUp() {
	expCurrentCap := int64(1)
	expNewCap := int64(3)
	minSize := int64(s.AWSScaler.spec.DefaultMinWorkerNodes)
	maxSize := int64(s.AWSScaler.spec.DefaultMaxWorkerNodes)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
		DesiredCapacity:      &expNewCap,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)
	s.AwsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	currentCap, newCap, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), 2)
	s.Require().NoError(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())

	s.Equal(uint64(expCurrentCap), currentCap)
	s.Equal(uint64(3), newCap)
}

func (s *AwsScalerTestSuite) Test_ScaleWorkerByDelta_ScalesDown() {
	expCurrentCap := int64(4)
	expNewCap := int64(1)
	minSize := int64(s.AWSScaler.spec.DefaultMinWorkerNodes)
	maxSize := int64(s.AWSScaler.spec.DefaultMaxWorkerNodes)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
		DesiredCapacity:      &expNewCap,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)
	s.AwsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, nil)

	currentCap, newCap, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), -3)
	s.Require().NoError(err)
	s.AwsAutoScalerMock.AssertExpectations(s.T())

	s.Equal(uint64(expCurrentCap), currentCap)
	s.Equal(uint64(1), newCap)
}

func (s *AwsScalerTestSuite) Test_ScaleManagerByDelta_UpdateAutoScalingError() {
	expCurrentCap := int64(4)
	expNewCap := int64(1)
	minSize := int64(s.AWSScaler.spec.DefaultMinWorkerNodes)
	maxSize := int64(s.AWSScaler.spec.DefaultMaxWorkerNodes)
	describeOutput := autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			&autoscaling.Group{
				AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
				DesiredCapacity:      &expCurrentCap,
			},
		},
	}
	autoScaleInput := autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &s.AWSScaler.spec.WorkerASG,
		DesiredCapacity:      &expNewCap,
		MinSize:              &minSize,
		MaxSize:              &maxSize,
	}
	autoScaleOutput := autoscaling.UpdateAutoScalingGroupOutput{}
	autoScaleError := errors.New("Auto scaling error")

	s.AwsAutoScalerMock.On("DescribeAutoScalingGroupsWithContext", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.AnythingOfType("[]request.Option")).Return(&describeOutput, nil)
	s.AwsAutoScalerMock.
		On("UpdateAutoScalingGroupWithContext", mock.AnythingOfType("*context.emptyCtx"), &autoScaleInput, mock.AnythingOfType("[]request.Option")).
		Return(&autoScaleOutput, autoScaleError)

	_, _, err := s.AWSScaler.ScaleWorkerByDelta(context.Background(), -3)
	s.Error(err)
	s.Contains(err.Error(), "Error calling update-auto-scaling-group")
	s.AwsAutoScalerMock.AssertExpectations(s.T())

}

type AwsAutoScalerMock struct {
	mock.Mock
}

func (m *AwsAutoScalerMock) DescribeAutoScalingGroupsWithContext(ctx aws.Context, input *autoscaling.DescribeAutoScalingGroupsInput, opts ...request.Option) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), args.Error(1)
}

func (m *AwsAutoScalerMock) UpdateAutoScalingGroupWithContext(ctx aws.Context, input *autoscaling.UpdateAutoScalingGroupInput, opts ...request.Option) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*autoscaling.UpdateAutoScalingGroupOutput), args.Error(1)
}
