package heuristics

import (
	"context"
	"fmt"
	"strings"
	"time"

	internalconfig "github.com/DrSkyle/cloudslash/v2/pkg/config"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
)

// IdleClusterHeuristic checks idle clusters.
type IdleClusterHeuristic struct {
	Config internalconfig.IdleClusterConfig
}

// Name returns the name of the heuristic.
func (h *IdleClusterHeuristic) Name() string { return "IdleClusterHeuristic" }

// Run executes the heuristic analysis.
func (h *IdleClusterHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var clusters []*graph.Node
	// Index instances.
	instancesByCluster := make(map[string][]*graph.Node)

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::ECS::Cluster" {
			clusters = append(clusters, node)
		}
		if node.TypeStr() == "AWS::ECS::ContainerInstance" {
			if clusterArn, ok := node.Properties["ClusterArn"].(string); ok {
				instancesByCluster[clusterArn] = append(instancesByCluster[clusterArn], node)
			}
		}
	}
	g.Mu.RUnlock()

	for _, cluster := range clusters {

		// Check 1: Capacity validation.
		regInstances, _ := cluster.Properties["RegisteredContainerInstancesCount"].(int)
		if regInstances == 0 {
			// Skip empty.
			continue
		}

		// 2. Check workload.
		runningTasks, _ := cluster.Properties["RunningTasksCount"].(int)
		pendingTasks, _ := cluster.Properties["PendingTasksCount"].(int)

		if runningTasks > 0 || pendingTasks > 0 {
			continue
		}

		// 3. Check services.
		activeServices, _ := cluster.Properties["ActiveServicesCount"].(int)
		if activeServices > 0 {

			continue
		}

		// Check uptime.
		isWaste := true

		instances := instancesByCluster[cluster.IDStr()]

		uptimeThreshold := h.Config.UptimeThreshold
		if uptimeThreshold == 0 {
			uptimeThreshold = 1 * time.Hour // Default
		}



		if len(instances) == 0 {
			// Skip inconsistent.
			isWaste = false
		} else {
			for _, inst := range instances {
				registeredAt, ok := inst.Properties["RegisteredAt"].(time.Time)
				if ok {
					if time.Since(registeredAt) > uptimeThreshold {
						// Stable.
					} else {
						// Fresh instance.
						isWaste = false
						break
					}
				}
			}

			// Idle.
		}

		if isWaste {
			g.MarkWaste(cluster.IDStr(), 85)
			stats.ItemsFound++

			reason := fmt.Sprintf("Idle Cluster: %d active Container Instances (>1h uptime) with 0 running tasks.", regInstances)
			cluster.Properties["Reason"] = reason
		}
	}

	return stats, nil
}

// EmptyServiceHeuristic detects stuck services.
type EmptyServiceHeuristic struct {
	ECR *aws.ECRScanner
	ECS *aws.ECSScanner
}

// Name returns the name of the heuristic.
func (h *EmptyServiceHeuristic) Name() string { return "EmptyServiceHeuristic" }

// Run executes the heuristic analysis.
func (h *EmptyServiceHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var services []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::ECS::Service" {
			services = append(services, node)
		}
	}
	g.Mu.RUnlock()

	for _, service := range services {
		desired, _ := service.Properties["DesiredCount"].(int)
		running, _ := service.Properties["RunningCount"].(int)

		if desired > 0 && running == 0 {
			// Diagnose.
			events, _ := service.Properties["Events"].([]string)
			diagnosis := "Reason: Service is failing to start tasks."

			// Analyze events.
			for _, event := range events {
				if strings.Contains(event, "unable to place a task") {
					diagnosis = "Reason: Insufficient Capacity (Infrastructure Issue)."
					break
				}
				if strings.Contains(event, "task failed to start") {
					diagnosis = "Reason: Application Crash (Code Issue)."
					break
				}
				if strings.Contains(event, "deregistered target") {
					diagnosis = "Reason: Health Check Failure."
					break
				}
				if strings.Contains(event, "PullImage") || strings.Contains(event, "pull image") || strings.Contains(event, "ImageNotFound") {
					diagnosis = "Reason: Image Pull Failure."
				}
			}

			// Check ECR.
			if h.ECS != nil && h.ECR != nil {
				taskDefARN, _ := service.Properties["TaskDefinition"].(string)
				if taskDefARN != "" {
					// Describe task.
					tdOut, err := h.ECS.Client.DescribeTaskDefinition(ctx, &awsecs.DescribeTaskDefinitionInput{
						TaskDefinition: &taskDefARN,
					})
					if err == nil && tdOut.TaskDefinition != nil && len(tdOut.TaskDefinition.ContainerDefinitions) > 0 {
						// Check container.
						imageURI := *tdOut.TaskDefinition.ContainerDefinitions[0].Image
						if imageURI != "" {
							// Verify existence.
							exists, err := h.ECR.CheckImageExists(ctx, imageURI)
							if err == nil && !exists {
								diagnosis = "Reason: Broken Artifact. Image not found in ECR."
							}
						}
					}
				}
			}

			g.MarkWaste(service.IDStr(), 90)
			service.Properties["Reason"] = fmt.Sprintf("STUCK Service. Desired: %d, Running: 0. %s", desired, diagnosis)
			stats.ItemsFound++
		}
	}

	return stats, nil
}
