package remediation

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// Generator creates remediation scripts.
type Generator struct {
	Graph *graph.Graph
}

// NewGenerator creates a new remediation generator.
func NewGenerator(g *graph.Graph) *Generator {
	return &Generator{Graph: g}
}

var idRegex = regexp.MustCompile("^[a-zA-Z0-9._-]+$")

// GenerateSafeDeleteScript creates a shell script for safe cleanup.
func (g *Generator) GenerateSafeDeleteScript(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Safe Remediation Script\n")
	fmt.Fprintf(f, "# Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "set -e\n\n")

	wasteCount := 0

	for _, node := range g.Graph.Nodes {
		if !node.IsWaste {
			continue
		}

		// Extract resource ID from ARN.
		resourceID := extractResourceID(node.ID)

		if !idRegex.MatchString(resourceID) {
			fmt.Fprintf(f, "# SKIPPING MALFORMED ID (Potential Injection): %s\n", resourceID)
			continue
		}

		switch node.Type {
		case "AWS::EC2::Instance":
			fmt.Fprintf(f, "echo \"Stopping EC2 Instance: %s\"\n", resourceID)
			// Generate soft delete command.
			fmt.Fprintf(f, "aws ec2 stop-instances --instance-ids %s\n\n", resourceID)
			wasteCount++


		case "AWS::EC2::Volume":
			fmt.Fprintf(f, "echo \"Processing Volume: %s\"\n", resourceID)
			
			// Safety check: EBS Modernization.
			if isGP2, _ := node.Properties["IsGP2"].(bool); isGP2 {
				fmt.Fprintf(f, "echo \"  -> ACTION: Modify Volume (Optimizing gp2 -> gp3)\"\n")
				fmt.Fprintf(f, "aws ec2 modify-volume --volume-id %s --volume-type gp3\n\n", resourceID)
			} else {
				// Standard Waste: Delete
				desc := fmt.Sprintf("CloudSlash-Archive-%s", resourceID)
				fmt.Fprintf(f, "aws ec2 create-snapshot --volume-id %s --description \"%s\" --tag-specifications 'ResourceType=snapshot,Tags=[{Key=CloudSlash,Value=Archive}]'\n", resourceID, desc)
				fmt.Fprintf(f, "aws ec2 delete-volume --volume-id %s\n\n", resourceID)
			}
			wasteCount++

		case "AWS::RDS::DBInstance":
			fmt.Fprintf(f, "echo \"Stopping RDS: %s\"\n", resourceID)
			// Generate stop command.
			fmt.Fprintf(f, "aws rds stop-db-instance --db-instance-identifier %s\n\n", resourceID)
			wasteCount++

		case "AWS::EC2::NatGateway":
			fmt.Fprintf(f, "echo \"Processing NAT Gateway: %s\"\n", resourceID)
			// Delete NAT Gateway.
			fmt.Fprintf(f, "aws ec2 delete-nat-gateway --nat-gateway-id %s\n\n", resourceID)
			wasteCount++

		case "AWS::EC2::EIP":
			fmt.Fprintf(f, "echo \"Processing EIP: %s\"\n", resourceID)
			// Release Elastic IP.
			fmt.Fprintf(f, "aws ec2 release-address --allocation-id %s\n\n", resourceID)
			wasteCount++
		
		case "AWS::EC2::AMI":
			fmt.Fprintf(f, "echo \"Deregistering AMI: %s\"\n", resourceID)
			// Deregister AMI
			fmt.Fprintf(f, "aws ec2 deregister-image --image-id %s\n\n", resourceID)
			wasteCount++

		case "AWS::ElasticLoadBalancingV2::LoadBalancer":
			fmt.Fprintf(f, "echo \"Deleting Load Balancer: %s\"\n", node.ID)
			fmt.Fprintf(f, "aws elbv2 delete-load-balancer --load-balancer-arn %s\n\n", node.ID)
			wasteCount++

		case "AWS::ECS::Cluster":
			fmt.Fprintf(f, "echo \"Deleting ECS Cluster: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ecs delete-cluster --cluster %s\n\n", resourceID)
			wasteCount++

		case "AWS::ECS::Service":
			parts := strings.Split(node.ID, "/")
			if len(parts) >= 3 {
				cluster := parts[1]
				service := parts[2]
				fmt.Fprintf(f, "echo \"Deleting ECS Service: %s/%s\"\n", cluster, service)
				fmt.Fprintf(f, "aws ecs delete-service --cluster %s --service %s --force\n\n", cluster, service)
				wasteCount++
			}

		case "AWS::EKS::Cluster":
			// ID for cluster is usually Name or ARN. Using resourceID (name)
			fmt.Fprintf(f, "echo \"Deleting EKS Cluster: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws eks delete-cluster --name %s\n\n", resourceID)
			wasteCount++

		case "AWS::EKS::NodeGroup":
			cluster := "unknown-cluster"
			if n, ok := node.Properties["ClusterName"].(string); ok {
				cluster = n
			}
			fmt.Fprintf(f, "echo \"Deleting EKS NodeGroup: %s (Cluster: %s)\"\n", resourceID, cluster)
			fmt.Fprintf(f, "aws eks delete-nodegroup --cluster-name %s --nodegroup-name %s\n\n", cluster, resourceID)
			wasteCount++

		case "AWS::ECR::Repository":
			fmt.Fprintf(f, "echo \"Deleting ECR Repository: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws ecr delete-repository --repository-name %s --force\n\n", resourceID)
			wasteCount++

		case "AWS::Lambda::Function":
			fmt.Fprintf(f, "echo \"Deleting Lambda Function: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws lambda delete-function --function-name %s\n\n", resourceID)
			wasteCount++

		case "AWS::Logs::LogGroup":
			fmt.Fprintf(f, "echo \"Deleting Log Group: %s\"\n", resourceID)
			fmt.Fprintf(f, "aws logs delete-log-group --log-group-name \"%s\"\n\n", resourceID)
			wasteCount++
		}
	}

	if wasteCount == 0 {
		fmt.Fprintf(f, "echo \"No waste found to remediate.\"\n")
	} else {
		fmt.Fprintf(f, "echo \"Safe Remediation Complete. %d resources processed.\"\n", wasteCount)
	}

	return nil
}

// GenerateIgnoreScript creates a script to tag resources as ignored.
func (g *Generator) GenerateIgnoreScript(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	g.Graph.Mu.RLock()
	defer g.Graph.Mu.RUnlock()

	fmt.Fprintf(f, "#!/bin/bash\n")
	fmt.Fprintf(f, "# CloudSlash Ignore Tagging Script\n")
	fmt.Fprintf(f, "# Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "# Run this script to suppressed reporting for these resources in future scans.\n")
	fmt.Fprintf(f, "set -e\n\n")

	fmt.Fprintf(f, "set -e\n\n")

	// Sort waste nodes for deterministic output.
	type wasteItem struct {
		ID   string
		Type string
	}
	var items []wasteItem

	for _, node := range g.Graph.Nodes {
		if node.IsWaste && !node.Justified {
			items = append(items, wasteItem{ID: node.ID, Type: node.Type})
		}
	}

	// Sort by ID
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	count := 0
	for _, item := range items {
		resourceID := extractResourceID(item.ID)

		arg := item.ID
		if !strings.HasPrefix(item.ID, "arn:") {
			fmt.Fprintf(f, "# Skipping non-ARN resource: %s\n", item.ID)
			continue
		}

		fmt.Fprintf(f, "echo \"Ignoring: %s\"\n", resourceID)
		fmt.Fprintf(f, "aws resourcegroupstaggingapi tag-resources --resource-arn-list %s --tags cloudslash:ignore=true\n", arg)
		count++
	}

	if count == 0 {
		fmt.Fprintf(f, "echo \"No waste found to ignore.\"\n")
	} else {
		fmt.Fprintf(f, "echo \"Ignore Tagging Complete. %d resources tagged.\"\n", count)
	}

	return nil
}

func extractResourceID(id string) string {
	// Parse ARN using official library.
	
	if parsed, err := arn.Parse(id); err == nil {
		// Use fields function to split by / or : safely
		parts := strings.FieldsFunc(parsed.Resource, func(r rune) bool {
			return r == '/' || r == ':'
		})
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return parsed.Resource
	}

	// Return original ID if not an ARN.
	return id
}
