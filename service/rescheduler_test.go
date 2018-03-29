package service

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/suite"
)

type ReschedulerTestSuite struct {
	suite.Suite
	workerNodes        int
	managerNodes       int
	reschedulerService *reschedulerService
	dClient            *client.Client
	serviceNames       []string
	envKey             string
}

func TestReshedulerUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ReschedulerTestSuite))
}

func (s *ReschedulerTestSuite) SetupSuite() {
	client, err := NewDockerClientFromEnv()
	if err != nil {
		s.T().Skipf("Unable to connect to Docker Client")
	}
	defer client.Close()
	info, err := client.Info(context.Background())
	if err != nil {
		s.T().Skipf("Unable to connect to Docker Client")
	}
	_, err = client.SwarmInspect(context.Background())
	if err != nil {
		s.T().Skipf("Docker process is not a part of a swarm")
	}

	rs, err := NewReschedulerService(
		client,
		"com.df.reschedule",
		"RESCHEDULE_DATE",
		time.Second,
		time.Second*2,
	)
	s.Error(err)

	rs, err = NewReschedulerService(
		client,
		"com.df.reschedule=true",
		"RESCHEDULE_DATE",
		time.Second,
		time.Second*2,
	)
	s.Require().NoError(err)

	s.reschedulerService = rs.(*reschedulerService)
	s.workerNodes = info.Swarm.Nodes - info.Swarm.Managers
	s.managerNodes = info.Swarm.Managers
	s.dClient = client
	s.serviceNames = []string{"web_test1", "web_test2"}
	s.envKey = "RESCHEDULE_DATE"
}

func (s *ReschedulerTestSuite) SetupTest() {

	for _, name := range s.serviceNames {
		cmd := fmt.Sprintf(`docker service create --name %s \
			--replicas 1 -d \
			-l com.df.reschedule=true \
			-e %s=%s \
			-e STUFF=%s \
			alpine:3.6 sleep 10000000`, name, s.envKey, name, name)
		_, err := exec.Command("/bin/sh", "-c", cmd).Output()
		if err != nil {
			s.T().Skipf("Unable to create service: %s", err.Error())
		}
	}
}

func (s *ReschedulerTestSuite) TearDownTest() {

	for _, name := range s.serviceNames {
		cmd := fmt.Sprintf(`docker service rm %s`, name)
		exec.Command("/bin/sh", "-c", cmd).Output()
	}
}

func (s *ReschedulerTestSuite) Test_equalTargetCountManager() {
	equalTarget, err := s.reschedulerService.equalTargetCount(s.managerNodes, true)
	s.Require().NoError(err)
	s.True(equalTarget)
}

func (s *ReschedulerTestSuite) Test_equalTargetCountWorker() {
	equalTarget, err := s.reschedulerService.equalTargetCount(s.workerNodes, false)
	s.Require().NoError(err)
	s.True(equalTarget)
}

func (s *ReschedulerTestSuite) Test_Reschedule_ServiceDoesNotExist() {
	err := s.reschedulerService.RescheduleService("doesnotexist", "value")
	s.Require().Error(err)
	s.Contains(err.Error(), "Unable to inspect service doesnotexist")
}

func (s *ReschedulerTestSuite) Test_RescheduleSingleService() {
	cmd := `docker service update \
			--label-add com.df.reschedule=true \
			--env-add hello=world \
			--env-add wow \
			--env-rm RESCHEDULE_DATE \
			-d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()
	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)
	err := s.reschedulerService.RescheduleService(s.serviceNames[0], value)
	s.Require().NoError(err)

	service, _, err := s.dClient.ServiceInspectWithRaw(
		context.Background(), s.serviceNames[0], types.ServiceInspectOptions{})
	s.Require().NoError(err)

	envList := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envList, envAdd)

	// Calling again will not change envList
	err = s.reschedulerService.RescheduleService(s.serviceNames[0], value)
	s.Require().NoError(err)

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), s.serviceNames[0], types.ServiceInspectOptions{})
	s.Require().NoError(err)

	newEnvList := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Equal(newEnvList, envList)

	s.Contains(newEnvList, "hello=world")
	s.Contains(newEnvList, "wow")
}

func (s *ReschedulerTestSuite) Test_RescheduleSingleServiceFalseLabel() {
	cmd := `docker service update \
			--label-add com.df.reschedule=false -d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()
	value := "HELLOWORLD"

	err := s.reschedulerService.RescheduleService(s.serviceNames[0], value)
	s.Require().Error(err)

	expStr := "web_test1 is not labeled with com.df.reschedule=true (com.df.reschedule=false)"
	s.Equal(expStr, err.Error())
}

func (s *ReschedulerTestSuite) Test_RescheduleSingleServiceUnTrueLabel() {
	cmd := `docker service update \
			--label-add com.df.reschedule=boo -d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()
	value := "HELLOWORLD"

	err := s.reschedulerService.RescheduleService(s.serviceNames[0], value)
	s.Require().Error(err)

	expStr := "web_test1 is not labeled with com.df.reschedule=true (com.df.reschedule=boo)"
	s.Equal(expStr, err.Error())
}

func (s *ReschedulerTestSuite) Test_RescheduleSingleServiceNoLabel() {
	cmd := `docker service update \
			--label-rm com.df.reschedule -d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()
	value := "HELLOWORLD"

	err := s.reschedulerService.RescheduleService(s.serviceNames[0], value)
	s.Require().Error(err)

	expStr := "web_test1 is not labeled with com.df.reschedule=true (no label)"
	s.Equal(expStr, err.Error())
}

func (s *ReschedulerTestSuite) Test_RescheduleWithTrueLabel() {
	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)

	err := s.reschedulerService.RescheduleAll(value)
	s.Require().NoError(err)

	for _, name := range s.serviceNames {

		service, _, err := s.dClient.ServiceInspectWithRaw(
			context.Background(), name, types.ServiceInspectOptions{})
		s.Require().NoError(err)

		envLists := service.Spec.TaskTemplate.ContainerSpec.Env
		s.Contains(envLists, envAdd)
	}
}

func (s *ReschedulerTestSuite) Test_RescheduleWithFalseLabel() {
	cmd := `docker service update \
			--label-add com.df.reschedule=false -d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()

	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)
	err := s.reschedulerService.RescheduleAll(value)
	s.Require().NoError(err)

	service, _, err := s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, fmt.Sprintf("%s=web_test1", s.envKey))
	s.NotContains(envLists, envAdd)

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test2", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists = service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, envAdd)
}

func (s *ReschedulerTestSuite) Test_RescheduleWithNoLabel() {
	cmd := `docker service update \
			--label-rm com.df.reschedule -d web_test1`
	exec.Command("/bin/sh", "-c", cmd).Output()

	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)
	err := s.reschedulerService.RescheduleAll(value)
	s.Require().NoError(err)

	service, _, err := s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, fmt.Sprintf("%s=web_test1", s.envKey))
	s.NotContains(envLists, envAdd)

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test2", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists = service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, envAdd)
}

func (s *ReschedulerTestSuite) Test_RescheduleChangesLabel() {
	value := "HELLOWORLDNEW"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)

	service, _, err := s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, fmt.Sprintf("%s=web_test1", s.envKey))

	err = s.reschedulerService.RescheduleAll(value)
	s.Require().NoError(err)

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists = service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, envAdd)
}

func (s *ReschedulerTestSuite) Test_RescheduleChangesLabelLeavesCurrentOnesAlone() {
	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)

	service, _, err := s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)
	envLists := service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, "STUFF=web_test1")

	err = s.reschedulerService.RescheduleAll(value)
	s.Require().NoError(err)

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test1", types.ServiceInspectOptions{})
	s.Require().NoError(err)

	envLists = service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, envAdd)
	s.Contains(envLists, "STUFF=web_test1")

	service, _, err = s.dClient.ServiceInspectWithRaw(
		context.Background(), "web_test2", types.ServiceInspectOptions{})
	s.Require().NoError(err)

	envLists = service.Spec.TaskTemplate.ContainerSpec.Env
	s.Contains(envLists, envAdd)
	s.Contains(envLists, "STUFF=web_test2")
}

func (s *ReschedulerTestSuite) Test_RescheduleWithTrueLabelWaitTicker() {
	value := "HELLOWORLD"
	envAdd := fmt.Sprintf("%s=%s", s.envKey, value)

	tickerC := make(chan time.Time)
	errC := make(chan error, 1)

	go s.reschedulerService.RescheduleServicesWaitForNodes(true, s.managerNodes, value, tickerC, errC)

	ticks := []time.Time{}
	var err error

L:
	for {
		select {
		case t := <-tickerC:
			ticks = append(ticks, t)
		case err = <-errC:
			close(tickerC)
			break L
		}
	}

	s.Require().NoError(err)
	s.Len(ticks, 1)

	for _, name := range s.serviceNames {

		service, _, err := s.dClient.ServiceInspectWithRaw(
			context.Background(), name, types.ServiceInspectOptions{})
		s.Require().NoError(err)

		envLists := service.Spec.TaskTemplate.ContainerSpec.Env
		s.Contains(envLists, envAdd)
	}
}

func (s *ReschedulerTestSuite) Test_RescheduleWithTrueLabelWaitTimeOut() {
	value := "HELLOWORLD"

	tickerC := make(chan time.Time)
	errC := make(chan error, 1)

	go s.reschedulerService.RescheduleServicesWaitForNodes(false, s.managerNodes, value, tickerC, errC)

	ticks := []time.Time{}
	var err error

L:
	for {
		select {
		case t := <-tickerC:
			ticks = append(ticks, t)
		case err = <-errC:
			close(tickerC)
			break L
		}
	}

	s.Require().Error(err)
	s.Len(ticks, 2)

}
