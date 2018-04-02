package cloud

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CloudTestSuite struct {
	suite.Suite
}

func TestCloudUnitTestSuite(t *testing.T) {
	suite.Run(t, new(CloudTestSuite))
}

func (s *CloudTestSuite) Test_NewCloudAWS_ENVFileDoesNotExist() {
	_, err := NewCloud("aws", NewCloudOptions{})
	s.Require().Error(err)
}

func (s *CloudTestSuite) Test_NewCloudAWS_NoAWSManagerASG() {
	_, err := NewCloud("aws", NewCloudOptions{})
	s.Require().Error(err)
}

func (s *CloudTestSuite) Test_NewCloudAWS() {
	defer func() {
		os.Unsetenv("AWS_MANAGER_ASG")
		os.Unsetenv("AWS_WORKER_ASG")
	}()
	os.Setenv("AWS_MANAGER_ASG", "awsmanager")
	os.Setenv("AWS_WORKER_ASG", "awsworker")

	c, err := NewCloud("aws", NewCloudOptions{})
	s.Require().NoError(err)
	s.NotNil(c)
}

func (s *CloudTestSuite) Test_NewCloud_UnrecognizedBackend() {
	c, err := NewCloud("what", NewCloudOptions{})
	s.Require().Error(err)
	s.Nil(c)

	s.Equal("backend what does not exist", err.Error())
}
