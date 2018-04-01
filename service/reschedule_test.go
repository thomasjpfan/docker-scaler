package service

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ReschedulerTestSuite struct {
	suite.Suite
	reschedulerService *reschedulerService
	clientMock         *DockerClientMock
	envKey             string
	ctx                context.Context
}

func TestReschedulerTestSuite(t *testing.T) {
	suite.Run(t, new(ReschedulerTestSuite))
}

func (s *ReschedulerTestSuite) SetupSuite() {
	s.envKey = "RESCHEDULE_DATE"
	s.ctx = context.Background()
}

func (s *ReschedulerTestSuite) SetupTest() {
	s.clientMock = new(DockerClientMock)
	rs, err := NewReschedulerService(
		s.clientMock,
		"com.df.reschedule=true",
		s.envKey,
		time.Second,
		time.Second*2,
	)
	s.Require().NoError(err)
	s.reschedulerService = rs.(*reschedulerService)
}

func (s *ReschedulerTestSuite) Test_Reschedule_ServiceDoesNotExist() {

	expErr := errors.New("DOESNOTEXIST does not exist")
	s.clientMock.On("ServiceInspect", s.ctx, "DOESNOTEXIST").
		Return(swarm.Service{}, expErr)

	err := s.reschedulerService.RescheduleService("DOESNOTEXIST", "value")
	s.Require().Error(err)
	s.Contains(err.Error(), "Unable to inspect service DOESNOTEXIST")
}

func (s *ReschedulerTestSuite) Test_Reschedule_ServiceWithoutFilterKey() {
	ts := s.getTestService()
	delete(ts.Spec.Annotations.Labels, "com.df.reschedule")
	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().Error(err)
	s.Equal("web_test is not labeled with com.df.reschedule=true (no label)", err.Error())

	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_Reschedule_ServiceWithoutFilterValue() {
	ts := s.getTestService()
	ts.Spec.Annotations.Labels["com.df.reschedule"] = "false"
	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().Error(err)
	s.Equal("web_test is not labeled with com.df.reschedule=true (com.df.reschedule=false)", err.Error())
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_UpdateFails() {
	ts := s.getTestService()
	expErr := errors.New("Failed")

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil).
		On("ServiceUpdate", s.ctx, ts.ID, ts.Version, ts.Spec).
		Return(expErr)

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().Error(err)

	s.Contains(err.Error(), "Unable to reschedule service")
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_EnvExists() {
	ts := s.getTestService()

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil).
		On("ServiceUpdate", s.ctx, ts.ID, ts.Version, ts.Spec).
		Return(nil)

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().NoError(err)

	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_NilContainerSpec() {
	ts := s.getTestService()
	ts.Spec.TaskTemplate.ContainerSpec = nil

	var newSpec swarm.ServiceSpec
	expSpec := s.getTestService().Spec
	expSpec.TaskTemplate.ContainerSpec.Env = []string{"RESCHEDULE_DATE=value"}

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil).
		On("ServiceUpdate", s.ctx, ts.ID, ts.Version, mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil).Run(func(args mock.Arguments) {
		newSpec = args.Get(3).(swarm.ServiceSpec)
	})

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().NoError(err)

	s.clientMock.AssertExpectations(s.T())
	s.Equal(expSpec, newSpec)
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_UpdatesValue() {
	ts := s.getTestService()
	ts.Spec.TaskTemplate.ContainerSpec.Env = []string{"RESCHEDULE_DATE=oldvalue"}

	var newSpec swarm.ServiceSpec
	expSpec := s.getTestService().Spec
	expSpec.TaskTemplate.ContainerSpec.Env = []string{"RESCHEDULE_DATE=value"}

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil).
		On("ServiceUpdate", s.ctx, ts.ID, ts.Version, mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil).Run(func(args mock.Arguments) {
		newSpec = args.Get(3).(swarm.ServiceSpec)
	})

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().NoError(err)

	s.clientMock.AssertExpectations(s.T())
	s.Equal(expSpec, newSpec)
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_SameValueDoesNotUpdate() {
	ts := s.getTestService()
	ts.Spec.TaskTemplate.ContainerSpec.Env = []string{"RESCHEDULE_DATE=value"}

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil)

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().NoError(err)

	s.clientMock.AssertExpectations(s.T())
	s.clientMock.AssertNotCalled(s.T(), "ServiceUpdate")
}

func (s *ReschedulerTestSuite) Test_Reschedule_Service_AddsNewValue() {
	ts := s.getTestService()
	ts.Spec.TaskTemplate.ContainerSpec.Env = []string{"RANDOMENV=1", "HELLO"}

	var newSpec swarm.ServiceSpec
	expSpec := s.getTestService().Spec
	expSpec.TaskTemplate.ContainerSpec.Env = []string{"RANDOMENV=1", "HELLO", "RESCHEDULE_DATE=value"}

	s.clientMock.On("ServiceInspect", s.ctx, "web_test").
		Return(ts, nil).
		On("ServiceUpdate", s.ctx, ts.ID, ts.Version, mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil).Run(func(args mock.Arguments) {
		newSpec = args.Get(3).(swarm.ServiceSpec)
	})

	err := s.reschedulerService.RescheduleService("web_test", "value")
	s.Require().NoError(err)

	s.clientMock.AssertExpectations(s.T())
	s.Equal(expSpec, newSpec)
}

func (s *ReschedulerTestSuite) Test_RescheduleAll_ListError() {
	serviceList := []swarm.Service{}
	expErr := errors.New("List error")

	s.clientMock.On("ServiceList", s.ctx, s.getFilter()).Return(serviceList, expErr)

	_, err := s.reschedulerService.RescheduleAll("value")
	s.Error(err)

}

func (s *ReschedulerTestSuite) Test_RescheduleAll_List() {
	ts1, ts2 := s.getTestService(), s.getTestService()
	ts2.ID = "web_testID2"
	ts2.Spec.Name = "web_test2"

	serviceList := []swarm.Service{ts1, ts2}
	s.clientMock.On("ServiceList", s.ctx, s.getFilter()).Return(serviceList, nil).
		On("ServiceUpdate", s.ctx, mock.AnythingOfType("string"), mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil)

	status, err := s.reschedulerService.RescheduleAll("value")
	s.Require().NoError(err)
	s.Regexp("(web_test|web_test2), (web_test|web_test2) rescheduled", status)
}

func (s *ReschedulerTestSuite) Test_RescheduleAll_UpdateErrors() {
	ts1, ts2 := s.getTestService(), s.getTestService()
	ts2.ID = "web_testID2"
	ts2.Spec.Name = "web_test2"

	serviceList := []swarm.Service{ts1, ts2}

	s.clientMock.On("ServiceList", s.ctx, s.getFilter()).Return(serviceList, nil).
		On("ServiceUpdate", s.ctx, mock.AnythingOfType("string"), mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(errors.New("update error"))

	_, err := s.reschedulerService.RescheduleAll("value")
	s.Require().Error(err)

	s.Regexp("(web_test|web_test2), (web_test|web_test2) failed to reschedule", err.Error())

}

func (s *ReschedulerTestSuite) Test_RescheduleAll_PartialErrors() {
	ts1, ts2 := s.getTestService(), s.getTestService()
	ts2.ID = "web_testID2"
	ts2.Spec.Name = "web_test2"

	serviceList := []swarm.Service{ts1, ts2}

	s.clientMock.On("ServiceList", s.ctx, s.getFilter()).Return(serviceList, nil).
		On("ServiceUpdate", s.ctx, "web_testID", mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(errors.New("update error")).
		On("ServiceUpdate", s.ctx, "web_testID2", mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil)

	_, err := s.reschedulerService.RescheduleAll("value")
	s.Require().Error(err)

	s.Equal("web_test failed to reschedule (web_test2 succeeded)", err.Error())
}

func (s *ReschedulerTestSuite) Test_RescheduleServicesWaitForNodes_InfoError() {
	info := s.getInfo()
	expErr := errors.New("Info error")
	s.clientMock.On("Info", s.ctx).Return(info, expErr)

	tickerC := make(chan time.Time)
	errorC := make(chan error)
	statusC := make(chan string)

	go s.reschedulerService.RescheduleServicesWaitForNodes(true, 3, "value", tickerC, errorC, statusC)

	var err error
L:
	for {
		select {
		case e := <-errorC:
			err = e
			break L
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-tickerC:
		}
	}

	s.Equal("Unable to get docker info for node count: Info error", err.Error())
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_RescheduleServicesWaitForNodes_ServiceListFail() {

	info := s.getInfo()

	s.clientMock.On("Info", s.ctx).Return(info, nil).
		On("ServiceList", s.ctx, s.getFilter()).Return([]swarm.Service{}, errors.New("update error"))

	tickerC := make(chan time.Time)
	errorC := make(chan error)
	statusC := make(chan string)

	go s.reschedulerService.RescheduleServicesWaitForNodes(true, 3, "value", tickerC, errorC, statusC)

	var err error
L:
	for {
		select {
		case e := <-errorC:
			err = e
			break L
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-tickerC:
		}
	}
	s.Equal("Unable to get service list to reschedule: update error", err.Error())
	s.clientMock.AssertExpectations(s.T())

}

func (s *ReschedulerTestSuite) Test_RescheduleServicesWaitForNodes_Manager() {

	info := s.getInfo()
	ts := s.getTestService()
	newInfo := s.getInfo()
	newInfo.Swarm.Managers = 4
	newInfo.Swarm.Nodes = 6

	s.clientMock.On("Info", s.ctx).Return(info, nil).Return(newInfo, nil).
		On("ServiceList", s.ctx, s.getFilter()).Return([]swarm.Service{ts}, nil).
		On("ServiceUpdate", s.ctx, mock.AnythingOfType("string"), mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil)

	tickerC := make(chan time.Time)
	errorC := make(chan error)
	statusC := make(chan string)

	go s.reschedulerService.RescheduleServicesWaitForNodes(true, 4, "value", tickerC, errorC, statusC)

	var status string
L:
	for {
		select {
		case e := <-errorC:
			if e != nil {
				s.Failf("Error raised", "Error was raised: %s", e.Error())
				return
			}
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-tickerC:
		case s := <-statusC:
			status = s
			break L
		}
	}
	s.Equal("web_test rescheduled", status)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_RescheduleServicesWaitForNodes_Worker() {

	info := s.getInfo()
	ts := s.getTestService()
	newInfo := s.getInfo()
	newInfo.Swarm.Managers = 3
	newInfo.Swarm.Nodes = 7

	s.clientMock.On("Info", s.ctx).Return(info, nil).Return(newInfo, nil).
		On("ServiceList", s.ctx, s.getFilter()).Return([]swarm.Service{ts}, nil).
		On("ServiceUpdate", s.ctx, mock.AnythingOfType("string"), mock.AnythingOfType("swarm.Version"),
			mock.AnythingOfType("swarm.ServiceSpec")).
		Return(nil)

	tickerC := make(chan time.Time)
	errorC := make(chan error)
	statusC := make(chan string)

	go s.reschedulerService.RescheduleServicesWaitForNodes(false, 4, "value", tickerC, errorC, statusC)

	var status string
L:
	for {
		select {
		case e := <-errorC:
			if e != nil {
				s.Failf("Error raised", "Error was raised: %s", e.Error())
				return
			}
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-tickerC:
		case s := <-statusC:
			status = s
			break L
		}
	}
	s.Equal("web_test rescheduled", status)
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) Test_RescheduleServicesWaitForNodes_Timeout() {

	info := s.getInfo()

	s.clientMock.On("Info", s.ctx).Return(info, nil)

	tickerC := make(chan time.Time)
	errorC := make(chan error)
	statusC := make(chan string)

	go s.reschedulerService.RescheduleServicesWaitForNodes(true, 4, "value", tickerC, errorC, statusC)

	var err error

L:
	for {
		select {
		case e := <-errorC:
			err = e
			break L
		case <-time.After(time.Second * 5):
			s.Fail("Timeout")
			return
		case <-tickerC:
		case <-statusC:
			s.Fail("Has status message")
			return
		}
	}
	s.Contains(err.Error(), "Waited")
	s.clientMock.AssertExpectations(s.T())
}

func (s *ReschedulerTestSuite) getTestService() swarm.Service {
	labels := map[string]string{
		"com.df.reschedule": "true",
	}
	replicas := uint64(2)
	return swarm.Service{
		ID: "web_testID",
		Meta: swarm.Meta{
			Version: swarm.Version{
				Index: uint64(1),
			}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   "web_test",
				Labels: labels,
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &replicas,
				},
			},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{
					Env: []string{},
				},
			},
		},
	}
}

func (s *ReschedulerTestSuite) getInfo() types.Info {
	return types.Info{
		Swarm: swarm.Info{
			Managers: 3,
			Nodes:    5,
		},
	}
}

func (s *ReschedulerTestSuite) getFilter() types.ServiceListOptions {
	labelFilter := filters.NewArgs()
	labelFilter.Add("label", "com.df.reschedule=true")
	return types.ServiceListOptions{Filters: labelFilter}
}
