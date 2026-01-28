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

// IdleClusterHeuristic detects ECS Clusters with active container instances but no running tasks.
type IdleClusterHeuristic struct {
	Config internalconfig.IdleClusterConfig
}

func (h *IdleClusterHeuristic) Name() string { return "IdleClusterHeuristic" }

func (h *IdleClusterHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var clusters []*graph.Node
	// Index instances by Cluster ARN.
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

		// Check 1: Capacity Exists
		regInstances, _ := cluster.Properties["RegisteredContainerInstancesCount"].(int)
		if regInstances == 0 {
			// Fargate-only or empty clusters invoke no direct EC2 costs.
			continue
		}

		// Check 2: Workload is Zero
		runningTasks, _ := cluster.Properties["RunningTasksCount"].(int)
		pendingTasks, _ := cluster.Properties["PendingTasksCount"].(int)

		if runningTasks > 0 || pendingTasks > 0 {
			continue
		}

		// Check 3: Services are Dormant
		activeServices, _ := cluster.Properties["ActiveServicesCount"].(int)
		if activeServices > 0 {
			// Active services indicate intent.
			continue
		}

		// Verify that all instances in the cluster have been uptime long enough to rule out scaling events.
		// If runningTasks == 0 AND Instance Uptime > Threshold, we consider it waste.
		isWaste := true

		instances := instancesByCluster[cluster.ID]
		
		uptimeThreshold := h.Config.UptimeThreshold
		if uptimeThreshold == 0 {
			uptimeThreshold = 1 * time.Hour // Default
		}

		// Logic: If ANY instance is fresh (< threshold), the cluster might be scaling up.
		// If ALL instances are old (> threshold), it is identified as waste.
		
		if len(instances) == 0 {
			// Inconsistency: regInstances > 0 but no nodes found. Safest is to skip.
			isWaste = false
		} else {
			for _, inst := range instances {
				registeredAt, ok := inst.Properties["RegisteredAt"].(time.Time)
				if ok {
					if time.Since(registeredAt) > uptimeThreshold {
						// Old instance found.
					} else {
						// Found a fresh instance. Abort waste flag.
						isWaste = false
						break
					}
				}
			}

				// All instances were checked and none were below the uptime threshold.
				// This confirms the cluster has been idle for at least the threshold duration.
		}

		if isWaste {
			g.MarkWaste(cluster.ID, 85)

			reason := fmt.Sprintf("Idle Cluster: %d active Container Instances (>1h uptime) with 0 running tasks.", regInstances)
			cluster.Properties["Reason"] = reason
		}
	}

	return nil
}

// EmptyServiceHeuristic detects services with a desired count > 0 but running count == 0.
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
			// Service is stuck. Attempt to diagnose the cause from events.
			events, _ := service.Properties["Events"].([]string)
			diagnosis := "Reason: Service is failing to start tasks."

			// Regex/String Match
			for _, event := range events {
				if strings.Contains(event, "unable to place a task") {
					diagnosis = "Reason: Insufficient Capacity (Infrastructure Issue)."
					break // strong signal
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

			// Check if the service failure is due to a missing ECR image.
			// Resolves the TaskDefinition to find the container image URI.
			if h.ECS != nil && h.ECR != nil {
				taskDefARN, _ := service.Properties["TaskDefinition"].(string)
				if taskDefARN != "" {
					// 1. Describe Task Definition to get Image URI.
					// Note: Only performing this for broken services to avoid N+1 issues.
					tdOut, err := h.ECS.Client.DescribeTaskDefinition(ctx, &awsecs.DescribeTaskDefinitionInput{
						TaskDefinition: &taskDefARN,
					})
					if err == nil && tdOut.TaskDefinition != nil && len(tdOut.TaskDefinition.ContainerDefinitions) > 0 {
						// Check first container image
						imageURI := *tdOut.TaskDefinition.ContainerDefinitions[0].Image
						if imageURI != "" {
							// 2. Check ECR
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
