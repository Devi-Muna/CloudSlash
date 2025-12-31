package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ECSScanner struct {
	Client *ecs.Client
	Graph  *graph.Graph
}

func NewECSScanner(cfg aws.Config, g *graph.Graph) *ECSScanner {
	return &ECSScanner{
		Client: ecs.NewFromConfig(cfg),
		Graph:  g,
	}
}

func (s *ECSScanner) ScanClusters(ctx context.Context) error {
	paginator := ecs.NewListClustersPaginator(s.Client, &ecs.ListClustersInput{})
	var clusterARNs []string

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		clusterARNs = append(clusterARNs, page.ClusterArns...)
	}

	if len(clusterARNs) == 0 {
		return nil
	}

	// DescribeClusters has a limit of 100 clusters per call.
	// We need to chunk the ARNs.
	chunkSize := 100
	for i := 0; i < len(clusterARNs); i += chunkSize {
		end := i + chunkSize
		if end > len(clusterARNs) {
			end = len(clusterARNs)
		}
		chunk := clusterARNs[i:end]

		output, err := s.Client.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: chunk,
			Include:  []types.ClusterField{types.ClusterFieldTags},
		})
		if err != nil {
			fmt.Printf("Error describing clusters: %v\n", err)
			continue
		}

		for _, cluster := range output.Clusters {
			s.addClusterNode(cluster)
			// For each cluster, we also want to scan its services and container instances
			if err := s.ScanServices(ctx, *cluster.ClusterArn); err != nil {
				fmt.Printf("Error scanning services for cluster %s: %v\n", *cluster.ClusterName, err)
			}
			if err := s.ScanContainerInstances(ctx, *cluster.ClusterArn); err != nil {
				fmt.Printf("Error scanning container instances for cluster %s: %v\n", *cluster.ClusterName, err)
			}
		}
	}

	return nil
}

func (s *ECSScanner) addClusterNode(cluster types.Cluster) {
	s.Graph.AddNode(*cluster.ClusterArn, "AWS::ECS::Cluster", map[string]interface{}{
			"Name":                              *cluster.ClusterName,
			"Status":                            *cluster.Status,
			"RegisteredContainerInstancesCount": int(cluster.RegisteredContainerInstancesCount),
			"RunningTasksCount":                 int(cluster.RunningTasksCount),
			"PendingTasksCount":                 int(cluster.PendingTasksCount),
			"ActiveServicesCount":               int(cluster.ActiveServicesCount),
	})
}

func (s *ECSScanner) ScanServices(ctx context.Context, clusterArn string) error {
	paginator := ecs.NewListServicesPaginator(s.Client, &ecs.ListServicesInput{
		Cluster: aws.String(clusterArn),
	})

	var serviceARNs []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		serviceARNs = append(serviceARNs, page.ServiceArns...)
	}

	if len(serviceARNs) == 0 {
		return nil
	}

	chunkSize := 10
	for i := 0; i < len(serviceARNs); i += chunkSize {
		end := i + chunkSize
		if end > len(serviceARNs) {
			end = len(serviceARNs)
		}
		chunk := serviceARNs[i:end]

		output, err := s.Client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: chunk,
		})
		if err != nil {
			fmt.Printf("Error describing services: %v\n", err)
			continue
		}

		for _, service := range output.Services {
			s.addServiceNode(service, clusterArn)
		}
	}
	return nil
}

func (s *ECSScanner) addServiceNode(service types.Service, clusterArn string) {
	events := []string{}
	// Capture last 3 events for forensics
	for i := 0; i < len(service.Events) && i < 3; i++ {
		events = append(events, *service.Events[i].Message)
	}

	// Get Task Definition ARN to link or inspect later (for image check)
	taskDef := ""
	if service.TaskDefinition != nil {
		taskDef = *service.TaskDefinition
	}

	s.Graph.AddNode(*service.ServiceArn, "AWS::ECS::Service", map[string]interface{}{
			"Name":           *service.ServiceName,
			"ClusterArn":     clusterArn,
			"Status":         *service.Status,
			"DesiredCount":   int(service.DesiredCount),
			"RunningCount":   int(service.RunningCount),
			"PendingCount":   int(service.PendingCount),
			"LaunchType":     string(service.LaunchType),
			"TaskDefinition": taskDef,
			"Events":         events,
	})
	s.Graph.AddTypedEdge(clusterArn, *service.ServiceArn, graph.EdgeTypeContains, 1)
}

func (s *ECSScanner) ScanContainerInstances(ctx context.Context, clusterArn string) error {
	paginator := ecs.NewListContainerInstancesPaginator(s.Client, &ecs.ListContainerInstancesInput{
		Cluster: aws.String(clusterArn),
	})

	var instanceARNs []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		instanceARNs = append(instanceARNs, page.ContainerInstanceArns...)
	}

	if len(instanceARNs) == 0 {
		return nil
	}

	chunkSize := 100
	for i := 0; i < len(instanceARNs); i += chunkSize {
		end := i + chunkSize
		if end > len(instanceARNs) {
			end = len(instanceARNs)
		}
		chunk := instanceARNs[i:end]

		output, err := s.Client.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
			Cluster:            aws.String(clusterArn),
			ContainerInstances: chunk,
		})
		if err != nil {
			continue
		}

		for _, ci := range output.ContainerInstances {
			// Add Container Instance Node
			// We map it to the underlying EC2 Instance ID if possible for the "Verification" uptime check
			ec2InstanceID := *ci.Ec2InstanceId
			
			// We primarily care about the registeredAt time for the "Waste" check
			// But we need to link it to the Cluster
			
			s.Graph.AddNode(*ci.ContainerInstanceArn, "AWS::ECS::ContainerInstance", map[string]interface{}{
					"ClusterArn":    clusterArn,
					"Ec2InstanceId": ec2InstanceID,
					"RegisteredAt":  ci.RegisteredAt,
					"Status":        *ci.Status,
			})
			s.Graph.AddTypedEdge(clusterArn, *ci.ContainerInstanceArn, graph.EdgeType("HAS_INSTANCE"), 1)
			
			// Link to EC2 Instance Node if exists (for cross-reference)
			ec2Arn := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", ec2InstanceID) 
			// Note: region/account are hard to guess here without context, but in heuristics we often match by suffix or property
			// Ideally we use a consistent ARN builder. For now, we store the EC2 ID in properties for the heuristic to look up.
			_ = ec2Arn 
		}
	}
	return nil
}
