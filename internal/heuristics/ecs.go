package heuristics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/aws"
	internalconfig "github.com/DrSkyle/cloudslash/internal/config"
	"github.com/DrSkyle/cloudslash/internal/graph"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
)

// IdleClusterHeuristic detects ECS Clusters that are costing money (EC2) but doing nothing.
type IdleClusterHeuristic struct {
	Config internalconfig.IdleClusterConfig
}

func (h *IdleClusterHeuristic) Name() string { return "IdleClusterHeuristic" }

func (h *IdleClusterHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var clusters []*graph.Node
	// Helper map to find instances by Cluster ARN
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
		// 1. The Filter (Indentifying the Zombie)

		// Check 1: Capacity Exists
		regInstances, _ := cluster.Properties["RegisteredContainerInstancesCount"].(int)
		if regInstances == 0 {
			// Fargate-only or empty. No EC2 waste directly attributed to "Idle Cluster".
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
			// Discussed: If ActiveServices > 0 but running tasks is 0, it might be broken services.
			// But for "Idle Cluster", we strictly want abandoned ones.
			continue
		}

		// Check 3: The Verification (The "Pro" Check)
		// Rule: If runningTasks == 0 AND Instance Uptime > Threshold, IT IS WASTE.
		isWaste := true

		instances := instancesByCluster[cluster.ID]
		hasOldInstances := false
		
		uptimeThreshold := h.Config.UptimeThreshold
		if uptimeThreshold == 0 {
			uptimeThreshold = 1 * time.Hour // Default
		}

		// Logic: If ANY instance is fresh (< threshold), the cluster might be scaling up.
		// If ALL instances are old (> threshold), it is waste.

		if len(instances) == 0 {
			// Inconsistency: regInstances > 0 but no nodes found. Safest is to skip.
			isWaste = false
		} else {
			for _, inst := range instances {
				registeredAt, ok := inst.Properties["RegisteredAt"].(time.Time)
				if ok {
					if time.Since(registeredAt) > uptimeThreshold {
						hasOldInstances = true
					} else {
						// Found a fresh instance. Abort waste flag.
						isWaste = false
						break
					}
				}
			}

			if !hasOldInstances && isWaste {
				// All instances were checked, none were old (and none were fresh? impossible if len > 0).
				// Wait. logic above:
				// if fresh found -> isWaste = false.
				// if old found -> hasOldInstances = true.
				// We need AT LEAST ONE old instance to confirm stasis?
				// Actually, if we didn't find any fresh instances, and we found *some* instances, they must be old.
				// So `isWaste` remains true (from init).
			}
		}

		if isWaste {
			g.MarkWaste(cluster.ID, 85)

			reason := fmt.Sprintf("Idle Cluster: %d Container Instances active (>1h uptime), but 0 Tasks running.", regInstances)
			cluster.Properties["Reason"] = reason
		}
	}

	return nil
}

// EmptyServiceHeuristic detects services that are trying to run but failing (Crash Loop).
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

		// The Filter
		if desired > 0 && running == 0 {
			// It's broken.

			// The Forensic Diagnosis
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

			// Broken Image Check (The "Genius Logic")
			// "inspect the TaskDefinition. Get the Image URI."
			if h.ECS != nil && h.ECR != nil {
				taskDefARN, _ := service.Properties["TaskDefinition"].(string)
				if taskDefARN != "" {
					// 1. Describe Task Definition to get Image URI
					// N+1 Alert: Only performing this for broken services.
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
								diagnosis = "Reason: 🚨 ZOMBIE (BROKEN ARTIFACT). Image not found in ECR."
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
