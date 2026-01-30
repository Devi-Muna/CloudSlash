package heuristics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/engine/aws"
	internalconfig "github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/graph"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
)

// IdleClusterHeuristic detects active clusters with no tasks.
type IdleClusterHeuristic struct {
	Config internalconfig.IdleClusterConfig
}

func (h *IdleClusterHeuristic) Name() string { return "IdleClusterHeuristic" }

func (h *IdleClusterHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var clusters []*graph.Node
	// Index instances.
	instancesByCluster := make(map[string][]*graph.Node)

	for _, node := range g.Nodes {
		if node.Type == "AWS::ECS::Cluster" {
			clusters = append(clusters, node)
		}
		if node.Type == "AWS::ECS::ContainerInstance" {
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
			// Skip empty/Fargate clusters.
			continue
		}

		// Check 2: Zero workload.
		runningTasks, _ := cluster.Properties["RunningTasksCount"].(int)
		pendingTasks, _ := cluster.Properties["PendingTasksCount"].(int)

		if runningTasks > 0 || pendingTasks > 0 {
			continue
		}

		// Check 3: Zero active services.
		activeServices, _ := cluster.Properties["ActiveServicesCount"].(int)
		if activeServices > 0 {
			// Active services imply intent.
			continue
		}

		// Verify instance uptime stability.
		isWaste := true

		instances := instancesByCluster[cluster.ID]
		
		uptimeThreshold := h.Config.UptimeThreshold
		if uptimeThreshold == 0 {
			uptimeThreshold = 1 * time.Hour // Default
		}

		// Check for recent scaling activity.
		// Confirm stability.
		
		if len(instances) == 0 {
			// Skip inconsistent state.
			isWaste = false
		} else {
			for _, inst := range instances {
				registeredAt, ok := inst.Properties["RegisteredAt"].(time.Time)
				if ok {
					if time.Since(registeredAt) > uptimeThreshold {
						// Instance stable.
					} else {
						// Fresh instance detected. Abort.
						isWaste = false
						break
					}
				}
			}

				// Cluster is stably idle.
		}

		if isWaste {
			g.MarkWaste(cluster.ID, 85)

			reason := fmt.Sprintf("Idle Cluster: %d active Container Instances (>1h uptime) with 0 running tasks.", regInstances)
			cluster.Properties["Reason"] = reason
		}
	}

	return nil
}

// EmptyServiceHeuristic detects stuck services.
type EmptyServiceHeuristic struct {
	ECR *aws.ECRScanner
	ECS *aws.ECSScanner
}

func (h *EmptyServiceHeuristic) Name() string { return "EmptyServiceHeuristic" }

func (h *EmptyServiceHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var services []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::ECS::Service" {
			services = append(services, node)
		}
	}
	g.Mu.RUnlock()

	for _, service := range services {
		desired, _ := service.Properties["DesiredCount"].(int)
		running, _ := service.Properties["RunningCount"].(int)

		if desired > 0 && running == 0 {
			// Diagnose stuck service.
			events, _ := service.Properties["Events"].([]string)
			diagnosis := "Reason: Service is failing to start tasks."

			// Analyze failure events.
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

			// Check for missing ECR image.
			if h.ECS != nil && h.ECR != nil {
				taskDefARN, _ := service.Properties["TaskDefinition"].(string)
				if taskDefARN != "" {
					// Describe Task Definition.
					// Optimization: Only check broken services.
					tdOut, err := h.ECS.Client.DescribeTaskDefinition(ctx, &awsecs.DescribeTaskDefinitionInput{
						TaskDefinition: &taskDefARN,
					})
					if err == nil && tdOut.TaskDefinition != nil && len(tdOut.TaskDefinition.ContainerDefinitions) > 0 {
						// Check primary container.
						imageURI := *tdOut.TaskDefinition.ContainerDefinitions[0].Image
						if imageURI != "" {
							// Verify ECR image existence.
							exists, err := h.ECR.CheckImageExists(ctx, imageURI)
							if err == nil && !exists {
								diagnosis = "Reason: Broken Artifact. Image not found in ECR."
							}
						}
					}
				}
			}

			g.MarkWaste(service.ID, 90)
			service.Properties["Reason"] = fmt.Sprintf("STUCK Service. Desired: %d, Running: 0. %s", desired, diagnosis)
		}
	}

	return nil
}
