package graph

import (
	"time"
)

// MockFactory constructs graph scenarios for testing.
type MockFactory struct {
	Graph *Graph
}

func NewMockFactory() *MockFactory {
	return &MockFactory{
		Graph: NewGraph(),
	}
}

func (m *MockFactory) AddInstance(id string, state string, launchAge time.Duration) {
	props := map[string]interface{}{
		"State":      state,
		"LaunchTime": time.Now().Add(-launchAge),
	}
	m.Graph.AddNode(id, "AWS::EC2::Instance", props)
}

func (m *MockFactory) AddVolume(id string, size int, attachedTo string) {
	props := map[string]interface{}{
		"State": "available",
		"Size":  size,
	}
	if attachedTo != "" {
		props["State"] = "in-use"
		props["AttachedInstanceId"] = attachedTo
		m.Graph.AddEdge(id, "arn:aws:ec2:region:account:instance/"+attachedTo)
	}
	m.Graph.AddNode(id, "AWS::EC2::Volume", props)
}

func (m *MockFactory) AddCluster(id string, runningTasks, pendingTasks, activeServices, containerInstances int) {
	props := map[string]interface{}{
		"RunningTasksCount":                 runningTasks,
		"PendingTasksCount":                 pendingTasks,
		"ActiveServicesCount":               activeServices,
		"RegisteredContainerInstancesCount": containerInstances,
	}
	m.Graph.AddNode(id, "AWS::ECS::Cluster", props)
}

func (m *MockFactory) AddContainerInstance(id, clusterArn string, age time.Duration) {
	props := map[string]interface{}{
		"ClusterArn":   clusterArn,
		"RegisteredAt": time.Now().Add(-age),
	}
	m.Graph.AddNode(id, "AWS::ECS::ContainerInstance", props)
}
