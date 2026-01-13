package remediation

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// Generator handles creates remediation scripts.
type Generator struct {
	Graph *graph.Graph
}

// NewGenerator creates a new remediation generator.
func NewGenerator(g *graph.Graph) *Generator {
	return &Generator{Graph: g}
}

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
	fmt.Fprintf(f, "set -e\n\n") // Exit on error

	wasteCount := 0

	for _, node := range g.Graph.Nodes {
		if !node.IsWaste {
			continue
		}

		// Resource ID extraction using robust ARN parsing
		resourceID := extractResourceID(node.ID)

		switch node.Type {
		case "AWS::EC2::Instance":
			fmt.Fprintf(f, "echo \"Stopping EC2 Instance: %s\"\n", resourceID)
			// Soft Delete: Stop the instance to stop compute billing, but keep storage.
			fmt.Fprintf(f, "aws ec2 stop-instances --instance-ids %s\n\n", resourceID)
			wasteCount++

		case "AWS::EC2::Volume":
			fmt.Fprintf(f, "echo \"Processing Volume: %s\"\n", resourceID)
			// Safety Snapshot
			desc := fmt.Sprintf("CloudSlash-Archive-%s", resourceID)
			fmt.Fprintf(f, "aws ec2 create-snapshot --volume-id %s --description \"%s\" --tag-specifications 'ResourceType=snapshot,Tags=[{Key=CloudSlash,Value=Archive}]'\n", resourceID, desc)
			// Detach if attached? Script logic implies unused, so just delete/archive.
			// Ideally we DetachVolume but for 'Waste' volumes they are usually unattached.
			fmt.Fprintf(f, "aws ec2 delete-volume --volume-id %s\n\n", resourceID)
			wasteCount++

		case "AWS::RDS::DBInstance":
			fmt.Fprintf(f, "echo \"Stopping RDS: %s\"\n", resourceID)
			// Soft Delete: Stop the DB
			fmt.Fprintf(f, "aws rds stop-db-instance --db-instance-identifier %s\n\n", resourceID)
			wasteCount++

		case "AWS::EC2::NatGateway":
			fmt.Fprintf(f, "echo \"Processing NAT Gateway: %s\"\n", resourceID)
			// Hard Delete as Stop is not supported.
			// Restoration logic will handle re-creation.
			fmt.Fprintf(f, "aws ec2 delete-nat-gateway --nat-gateway-id %s\n\n", resourceID)
			wasteCount++

		case "AWS::EC2::EIP":
			fmt.Fprintf(f, "echo \"Processing EIP: %s\"\n", resourceID)
			// Release
			fmt.Fprintf(f, "aws ec2 release-address --allocation-id %s\n\n", resourceID)
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

	// Collect waste nodes first to sort them for deterministic output
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
	// Robust ARN parsing using official library
	// This helps avoid fragile string splitting errors
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

	// Fallback for non-ARN inputs (e.g. raw IDs)
	return id
}
