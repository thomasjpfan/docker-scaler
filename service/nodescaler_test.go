package service

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/thomasjpfan/docker-scaler/service/cloud"
)

type CloudProviderMock struct {
	mock.Mock
}

func (m *CloudProviderMock) GetNodes(ctx context.Context, nodeType cloud.NodeType) (uint64, error) {
	args := m.Called(ctx, nodeType)
	return args.Get(0).(uint64), args.Error(1)
}
func (m *CloudProviderMock) SetNodes(ctx context.Context, nodeType cloud.NodeType, cnt, minSize, maxSize uint64) error {
	args := m.Called(ctx, nodeType, cnt, minSize, maxSize)
	return args.Error(0)
}
func (m *CloudProviderMock) String() string {
	return "cloudmock"
}

type InspectorMock struct {
	mock.Mock
}

func (m *InspectorMock) ServiceInspect(ctx context.Context, serviceID string) (swarm.Service, error) {
	args := m.Called(ctx, serviceID)
	return args.Get(0).(swarm.Service), args.Error(1)
}

type NodeScalerTestSuite struct {
	suite.Suite
	cloudProviderMock *CloudProviderMock
	inspectorMock     *InspectorMock
	nodeScaler        *NodeScaler
	managerOpts       ResolveDeltaOptions
	workerOpts        ResolveDeltaOptions
	ctx               context.Context
}

func TestNodeScalerNewUnitTest(t *testing.T) {
	suite.Run(t, new(NodeScalerTestSuite))
}

func (s *NodeScalerTestSuite) SetupTest() {
	s.cloudProviderMock = new(CloudProviderMock)
	s.inspectorMock = new(InspectorMock)
	s.managerOpts = ResolveDeltaOptions{
		MinLabel:           "com.df.scaleManagerNodeMin",
		MaxLabel:           "com.df.scaleManagerNodeMax",
		ScaleDownByLabel:   "com.df.scaleManagerNodeDownBy",
		ScaleUpByLabel:     "com.df.scaleManagerNodeUpBy",
		DefaultMin:         3,
		DefaultMax:         9,
		DefaultScaleDownBy: 1,
		DefaultScaleUpBy:   2,
	}
	s.workerOpts = ResolveDeltaOptions{
		MinLabel:           "com.df.scaleWorkerNodeMin",
		MaxLabel:           "com.df.scaleWorkerNodeMax",
		ScaleDownByLabel:   "com.df.scaleWorkerNodeDownBy",
		ScaleUpByLabel:     "com.df.scaleWorkerNodeUpBy",
		DefaultMin:         0,
		DefaultMax:         5,
		DefaultScaleDownBy: 2,
		DefaultScaleUpBy:   1,
	}

	s.nodeScaler = &NodeScaler{
		cloudProvider: s.cloudProviderMock,
		inspector:     s.inspectorMock,
		managerOpts:   s.managerOpts,
		workerOpts:    s.workerOpts,
	}
	s.ctx = context.Background()
}

func (s *NodeScalerTestSuite) TearDownTest() {
	s.inspectorMock.AssertExpectations(s.T())
	s.cloudProviderMock.AssertExpectations(s.T())
}

func (s *NodeScalerTestSuite) getNodeService() swarm.Service {
	labels := map[string]string{
		"com.df.scaleManagerNodeMin":    "5",
		"com.df.scaleManagerNodeMax":    "11",
		"com.df.scaleManagerNodeDownBy": "2",
		"com.df.scaleManagerNodeUpBy":   "1",
		"com.df.scaleWorkerNodeMin":     "1",
		"com.df.scaleWorkerNodeMax":     "7",
		"com.df.scaleWorkerNodeDownBy":  "1",
		"com.df.scaleWorkerNodeUpBy":    "2",
	}
	return swarm.Service{
		ID: "node_monitorID",
		Meta: swarm.Meta{
			Version: swarm.Version{
				Index: uint64(1),
			}},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   "node_monitor",
				Labels: labels,
			},
			Mode: swarm.ServiceMode{
				Global: &swarm.GlobalService{},
			},
		},
	}
}

func (s *NodeScalerTestSuite) Test_Scale_InspectError() {
	expErr := errors.New("docker inspect error")
	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(swarm.Service{}, expErr)

	_, _, err := s.nodeScaler.Scale(s.ctx, 0, ScaleUpDirection, cloud.NodeManagerType, "node_monitor")
	s.Require().Error(err)
	s.Equal("node scaling failed: docker inspect error", err.Error())
}

func (s *NodeScalerTestSuite) Test_Scale_CloudGetNodesError() {
	expErr := errors.New("cloud get error")
	s.cloudProviderMock.On("GetNodes", s.ctx, cloud.NodeWorkerType).
		Return(uint64(0), expErr)

	_, _, err := s.nodeScaler.Scale(s.ctx, 0, ScaleDownDirection, cloud.NodeWorkerType, "")
	s.Require().Error(err)
	s.Equal("node scaling failed: cloud get error", err.Error())
}

func (s *NodeScalerTestSuite) Test_Scale_CloudSetNodesError() {

	currentNodes := uint64(5)
	newNodes := uint64(4)
	expErr := errors.New("cloud update error")
	s.cloudProviderMock.On("GetNodes", s.ctx, cloud.NodeManagerType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, cloud.NodeManagerType,
			newNodes, s.managerOpts.DefaultMin, s.managerOpts.DefaultMax).
		Return(expErr)

	_, _, err := s.nodeScaler.Scale(s.ctx, 0, ScaleDownDirection, cloud.NodeManagerType, "")
	s.Require().Error(err)
	s.Equal("node scaling failed: cloud update error", err.Error())
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromQuery_LowerBoundFromService() {
	nodeType := cloud.NodeManagerType

	currentNodes := uint64(6)
	newNodes := uint64(5)

	expMinNodes := uint64(5)
	expMaxNodes := uint64(11)

	ns := s.getNodeService()
	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 2, ScaleDownDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromQuery_LowerBoundFromDefault() {
	nodeType := cloud.NodeWorkerType

	currentNodes := uint64(2)
	newNodes := uint64(0)

	expMinNodes := uint64(0)
	expMaxNodes := uint64(7)

	ns := s.getNodeService()

	delete(ns.Spec.Labels, "com.df.scaleWorkerNodeMin")

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 3, ScaleDownDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromQuery_UpperBoundFromService() {
	nodeType := cloud.NodeWorkerType

	currentNodes := uint64(5)
	newNodes := uint64(7)

	expMinNodes := uint64(1)
	expMaxNodes := uint64(7)

	ns := s.getNodeService()

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 3, ScaleUpDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromQuery_UpperBoundFromDefault() {
	nodeType := cloud.NodeManagerType

	currentNodes := uint64(7)
	newNodes := uint64(9)

	expMinNodes := uint64(5)
	expMaxNodes := uint64(9)

	ns := s.getNodeService()

	delete(ns.Spec.Labels, "com.df.scaleManagerNodeMax")

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 3, ScaleUpDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleUp_ByFromQuery() {
	nodeType := cloud.NodeWorkerType

	currentNodes := uint64(4)
	newNodes := uint64(6)

	expMinNodes := uint64(1)
	expMaxNodes := uint64(7)

	ns := s.getNodeService()

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 2, ScaleUpDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleUp_ByFromService() {
	nodeType := cloud.NodeManagerType

	currentNodes := uint64(6)
	newNodes := uint64(7)

	expMinNodes := uint64(5)
	expMaxNodes := uint64(11)

	ns := s.getNodeService()

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 0, ScaleUpDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleUp_ByFromDefault() {
	nodeType := cloud.NodeWorkerType

	currentNodes := uint64(3)
	newNodes := uint64(4)

	expMinNodes := uint64(1)
	expMaxNodes := uint64(5)

	ns := s.getNodeService()

	delete(ns.Spec.Labels, "com.df.scaleWorkerNodeMax")
	delete(ns.Spec.Labels, "com.df.scaleWorkerNodeUpBy")

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 0, ScaleUpDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromQuery() {

	nodeType := cloud.NodeManagerType

	currentNodes := uint64(10)
	newNodes := uint64(6)

	expMinNodes := uint64(5)
	expMaxNodes := uint64(11)

	ns := s.getNodeService()

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 4, ScaleDownDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromService() {

	nodeType := cloud.NodeWorkerType

	currentNodes := uint64(6)
	newNodes := uint64(5)

	expMinNodes := uint64(1)
	expMaxNodes := uint64(7)

	ns := s.getNodeService()

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 0, ScaleDownDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}

func (s *NodeScalerTestSuite) Test_ScaleDown_ByFromDefault() {

	nodeType := cloud.NodeManagerType

	currentNodes := uint64(7)
	newNodes := uint64(6)

	expMinNodes := uint64(5)
	expMaxNodes := uint64(11)

	ns := s.getNodeService()

	delete(ns.Spec.Labels, "com.df.scaleManagerNodeDownBy")

	s.inspectorMock.On("ServiceInspect", s.ctx, "node_monitor").
		Return(ns, nil)
	s.cloudProviderMock.On("GetNodes", s.ctx, nodeType).
		Return(currentNodes, nil).
		On("SetNodes", s.ctx, nodeType,
			newNodes, expMinNodes, expMaxNodes).
		Return(nil)

	nodesBefore, nodesNow, err := s.nodeScaler.Scale(s.ctx, 0, ScaleDownDirection, nodeType, "node_monitor")
	s.Require().NoError(err)

	s.Equal(currentNodes, nodesBefore)
	s.Equal(newNodes, nodesNow)
}
