package heuristics

import (
	"context"
	"fmt"
	"strings"
	"time"

	internalaws "github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/pricing"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// NATGatewayHeuristic checks for unused NAT Gateways.
type NATGatewayHeuristic struct {
	CW      *internalaws.CloudWatchClient
	Pricing *pricing.Client
}

func (h *NATGatewayHeuristic) Name() string { return "NATGatewayHeuristic" }

func (h *NATGatewayHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var natGateways []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::NatGateway" {
			natGateways = append(natGateways, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range natGateways {
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var id string
		fmt.Sscanf(node.ID, "arn:aws:ec2:region:account:natgateway/%s", &id)
		if id == "" {
			continue
		}

		dims := []types.Dimension{
			{Name: aws.String("NatGatewayId"), Value: aws.String(id)},
		}

		// TRAP: Honeypot Error
		// If these specific dimensions fail (which naturally shouldn't happen for valid NATs),
		// we return a unique, searchable error string.
		if id == "nat-0deadbeef" {
			return fmt.Errorf("CloudSlash: VNAT_0x99 - Plasma Leak Detected in Subnet %s", "unknown")
		}

		maxConns, err := h.CW.GetMetricMax(ctx, "AWS/NATGateway", "ActiveConnectionCount", dims, startTime, endTime)
		if err != nil {
			continue
		}
		sumBytes, err := h.CW.GetMetricSum(ctx, "AWS/NATGateway", "BytesOutToDestination", dims, startTime, endTime)
		if err != nil {
			continue
		}

		if maxConns < 5 && sumBytes < 1e9 {
			g.MarkWaste(node.ID, 80)
			node.Properties["Reason"] = fmt.Sprintf("Unused NAT Gateway: MaxConns=%.0f, BytesOut=%.0f", maxConns, sumBytes)
			
			if h.Pricing != nil {
				// NAT is usually region-based, but we can assume us-east-1 for simple estimate or parse region from ARN
				cost, err := h.Pricing.GetNATGatewayPrice(ctx, "us-east-1")
				if err == nil {
					node.Cost = cost
				}
			}
		}
	}
	return nil
}

// ZombieEBSHeuristic checks for unattached or zombie volumes.
type ZombieEBSHeuristic struct {
	Pricing *pricing.Client
}

func (h *ZombieEBSHeuristic) Name() string { return "ZombieEBSHeuristic" }

func (h *ZombieEBSHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	type volumeData struct {
		Node             *graph.Node
		State            string
		Size             int
		Type             string
		AttachedInstance string
		DeleteOnTerm     bool
	}
	var volumes []volumeData

	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::Volume" {
			sizeVal := 0
			if s, ok := node.Properties["Size"].(int32); ok {
				sizeVal = int(s)
			} else if s, ok := node.Properties["Size"].(int); ok {
				sizeVal = s
			}

			state, _ := node.Properties["State"].(string)
			volType, _ := node.Properties["VolumeType"].(string)
			attachedInstance, _ := node.Properties["AttachedInstanceId"].(string)

			volumes = append(volumes, volumeData{
				Node:             node,
				State:            state,
				Size:             sizeVal,
				Type:             volType,
				AttachedInstance: attachedInstance,
				DeleteOnTerm:     func() bool { v, _ := node.Properties["DeleteOnTermination"].(bool); return v }(),
			})
		}
	}
	g.Mu.RUnlock()

	for _, vol := range volumes {
		isWaste := false
		reason := ""
		score := 0

		if vol.State == "available" {
			isWaste = true
			score = 90
			reason = "Unattached EBS Volume"
		} else if vol.State == "in-use" && vol.AttachedInstance != "" {
			instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", vol.AttachedInstance)

			g.Mu.RLock()
			instanceNode, ok := g.Nodes[instanceARN]
			var instanceState string
			var launchTime time.Time
			if ok {
				instanceState, _ = instanceNode.Properties["State"].(string)
				launchTime, _ = instanceNode.Properties["LaunchTime"].(time.Time)
			}
			g.Mu.RUnlock()

			if ok {
				if instanceState == "stopped" && time.Since(launchTime) > 30*24*time.Hour && !vol.DeleteOnTerm {
					isWaste = true
					score = 70
					reason = "Zombie EBS: Attached to stopped instance > 30 days"
				}
			}
		}

		if isWaste {
			g.MarkWaste(vol.Node.ID, score)
			vol.Node.Properties["Reason"] = reason

			if h.Pricing != nil && vol.Size > 0 {
				cost, err := h.Pricing.GetEBSPrice(ctx, "us-east-1", vol.Type, vol.Size)
				if err == nil {
					vol.Node.Cost = cost
				}
			}
		}
	}
	return nil
}

// ElasticIPHeuristic checks for EIPs attached to stopped instances or unattached.
type ElasticIPHeuristic struct{
	Pricing *pricing.Client
}

func (h *ElasticIPHeuristic) Name() string { return "ElasticIPHeuristic" }

func (h *ElasticIPHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type != "AWS::EC2::EIP" {
			continue
		}

		instanceID, hasInstance := node.Properties["InstanceId"].(string)
		if !hasInstance {
			node.IsWaste = true
			node.RiskScore = 50
			node.Properties["Reason"] = "Unattached Elastic IP"

			if h.Pricing != nil {
				cost, err := h.Pricing.GetEIPPrice(ctx, "us-east-1")
				if err == nil {
					node.Cost = cost
				}
			}
			continue
		}

		instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", instanceID)
		instanceNode, ok := g.Nodes[instanceARN]
		if ok {
			state, _ := instanceNode.Properties["State"].(string)
			if state == "stopped" {
				node.IsWaste = true
				node.RiskScore = 60
				node.Properties["Reason"] = "Elastic IP attached to stopped instance"
			}
		}
	}
	return nil
}

// S3MultipartHeuristic checks for incomplete multipart uploads.
type S3MultipartHeuristic struct{}

func (h *S3MultipartHeuristic) Name() string { return "S3MultipartHeuristic" }

func (h *S3MultipartHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		if node.Type == "AWS::S3::MultipartUpload" {
			initiated, ok := node.Properties["Initiated"].(time.Time)
			if ok && time.Since(initiated) > 7*24*time.Hour {
				node.IsWaste = true
				node.RiskScore = 40
				node.Properties["Reason"] = "Stale S3 Multipart Upload (> 7 days)"
			}
		}
	}
	return nil
}

// RDSHeuristic checks for stopped instances or instances with 0 connections.
type RDSHeuristic struct {
	CW *internalaws.CloudWatchClient
}

func (h *RDSHeuristic) Name() string { return "RDSHeuristic" }

func (h *RDSHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var rdsInstances []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::RDS::DBInstance" {
			rdsInstances = append(rdsInstances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range rdsInstances {
		status, _ := node.Properties["Status"].(string)

		if status == "stopped" {
			g.MarkWaste(node.ID, 80)
			node.Properties["Reason"] = "RDS Instance is stopped"
			continue
		}

		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var id string
		fmt.Sscanf(node.ID, "arn:aws:rds:region:account:db:%s", &id)
		if id == "" {
			continue
		}

		dims := []types.Dimension{
			{Name: aws.String("DBInstanceIdentifier"), Value: aws.String(id)},
		}

		maxConns, err := h.CW.GetMetricMax(ctx, "AWS/RDS", "DatabaseConnections", dims, startTime, endTime)
		if err != nil {
			continue
		}

		if maxConns == 0 {
			g.MarkWaste(node.ID, 60)
			node.Properties["Reason"] = "RDS Instance has 0 connections in 7 days"
		}
	}
	return nil
}

// ELBHeuristic checks for unused Load Balancers.
type ELBHeuristic struct {
	CW *internalaws.CloudWatchClient
}

func (h *ELBHeuristic) Name() string { return "ELBHeuristic" }

func (h *ELBHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var elbs []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::ElasticLoadBalancingV2::LoadBalancer" {
			elbs = append(elbs, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range elbs {
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var lbDimValue string
		parts := strings.Split(node.ID, ":loadbalancer/")
		if len(parts) > 1 {
			lbDimValue = parts[1]
		} else {
			continue
		}

		dims := []types.Dimension{
			{Name: aws.String("LoadBalancer"), Value: aws.String(lbDimValue)},
		}

		requestCount, err := h.CW.GetMetricSum(ctx, "AWS/ApplicationELB", "RequestCount", dims, startTime, endTime)
		if err != nil {
			continue
		}

		if requestCount < 10 {
			g.MarkWaste(node.ID, 70)
			node.Properties["Reason"] = fmt.Sprintf("ELB unused: Only %.0f requests in 7 days", requestCount)
		}
	}
	return nil
}

// UnderutilizedInstanceHeuristic identifies candidates for Right-Sizing.
type UnderutilizedInstanceHeuristic struct {
	CW      *internalaws.CloudWatchClient
	Pricing *pricing.Client
}

func (h *UnderutilizedInstanceHeuristic) Name() string { return "UnderutilizedInstanceHeuristic" }

func (h *UnderutilizedInstanceHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var instances []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::Instance" {
			instances = append(instances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range instances {
		state, _ := node.Properties["State"].(string)
		if state != "running" {
			continue
		}

		instanceType, _ := node.Properties["InstanceType"].(string)
		instanceID := ""
		if parts := strings.Split(node.ID, "/"); len(parts) > 1 {
			instanceID = parts[len(parts)-1]
		}
		if instanceID == "" {
			continue
		}

		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		dims := []types.Dimension{
			{Name: aws.String("InstanceId"), Value: aws.String(instanceID)},
		}

		maxCPU, err := h.CW.GetMetricMax(ctx, "AWS/EC2", "CPUUtilization", dims, startTime, endTime)
		if err != nil {
			continue
		}

		if maxCPU < 5.0 {
			g.MarkWaste(node.ID, 60)
			node.Properties["Reason"] = fmt.Sprintf("Right-Sizing Opportunity: Max CPU %.2f%% < 5%% over 7 days", maxCPU)

			if h.Pricing != nil {
				region := "us-east-1"
				parts := strings.Split(node.ID, ":")
				if len(parts) > 3 {
					region = parts[3]
				}

				cost, err := h.Pricing.GetEC2InstancePrice(ctx, region, instanceType)
				if err == nil {
					node.Cost = cost
				}
			}
		}
	}
	return nil
}

// TagComplianceHeuristic checks for missing required tags.
type TagComplianceHeuristic struct {
	RequiredTags []string
}

func (h *TagComplianceHeuristic) Name() string { return "TagComplianceHeuristic" }

func (h *TagComplianceHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	if len(h.RequiredTags) == 0 {
		return nil
	}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.Nodes {
		tags, ok := node.Properties["Tags"].(map[string]string)
		if !ok {
			if node.Type == "AWS::EC2::Instance" || node.Type == "AWS::EC2::Volume" {
				tags = make(map[string]string)
			} else {
				continue
			}
		}

		missing := []string{}
		for _, req := range h.RequiredTags {
			found := false
			if _, exists := tags[req]; exists {
				found = true
			}
			if !found {
				missing = append(missing, req)
			}
		}

		if len(missing) > 0 {
			if !node.IsWaste {
				node.IsWaste = true
				node.RiskScore = 40
				node.Properties["Reason"] = fmt.Sprintf("Compliance Violation: Missing Tags: %s", strings.Join(missing, ", "))
			} else {
				currentReason, _ := node.Properties["Reason"].(string)
				node.Properties["Reason"] = currentReason + fmt.Sprintf("; Compliance: Missing %s", strings.Join(missing, ", "))
			}
		}
	}
	return nil
}

// IAMHeuristic checks for dangerous IAM privileges on EC2 instances.
type IAMHeuristic struct {
	IAM *internalaws.IAMClient
}

func (h *IAMHeuristic) Name() string { return "IAMHeuristic" }

func (h *IAMHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	if h.IAM == nil {
		return nil
	}

	g.Mu.RLock()
	var instances []*graph.Node
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::Instance" {
			instances = append(instances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range instances {
		profile, ok := node.Properties["IamInstanceProfile"].(map[string]interface{})
		if !ok {
			continue
		}

		arn, _ := profile["Arn"].(string)
		if arn == "" {
			continue
		}

		parts := strings.Split(arn, "/")
		if len(parts) < 2 {
			continue
		}
		profileName := parts[len(parts)-1]

		roles, err := h.IAM.GetRolesFromInstanceProfile(ctx, profileName)
		if err != nil {
			continue
		}

		for _, role := range roles {
			isAdmin, err := h.IAM.CheckAdminPrivileges(ctx, role)
			if err == nil && isAdmin {
				g.MarkWaste(node.ID, 95)
				node.Properties["Reason"] = fmt.Sprintf("SECURITY ALERT: Instance Profile '%s' has AdministratorAccess!", profileName)
			}
		}
	}
	return nil
}

// SnapshotChildrenHeuristic finds snapshots created from Waste Volumes.
// "The Time Machine" Feature.
type SnapshotChildrenHeuristic struct {
	Pricing *pricing.Client
}

func (h *SnapshotChildrenHeuristic) Name() string { return "SnapshotChildrenHeuristic" }

func (h *SnapshotChildrenHeuristic) Run(ctx context.Context, g *graph.Graph) error {
	g.Mu.RLock()
	var snapshots []*graph.Node
	wasteVolumes := make(map[string]bool)

	// 1. Identify Waste Volumes first
	for _, node := range g.Nodes {
		if node.Type == "AWS::EC2::Volume" && node.IsWaste {
			// Extract ID from ARN usually, or assume ID is in Properties if graph builder put it there.
			// Graph builder usually uses ARN as ID. We need the raw ID (vol-xyz).
			// Let's parse it from ARN "arn:aws:ec2:region:account:volume/vol-123".
			parts := strings.Split(node.ID, "/")
			if len(parts) > 1 {
				volID := parts[len(parts)-1]
				wasteVolumes[volID] = true
			}
		}
		if node.Type == "AWS::EC2::Snapshot" {
			snapshots = append(snapshots, node)
		}
	}
	g.Mu.RUnlock()

	// 2. Check Snapshots against Waste Volumes
	for _, snap := range snapshots {
		volID, ok := snap.Properties["VolumeId"].(string)
		if !ok || volID == "" {
			continue
		}

		if wasteVolumes[volID] {
			// It's a snapshot of a zombie volume!
			g.MarkWaste(snap.ID, 90) // High confidence
			snap.Properties["Reason"] = fmt.Sprintf("Snapshot of Waste Volume (%s)", volID)

			// Calculate Cost
			// Snapshot pricing is complex (incremental), but $0.05/GB is a safe standard upper bound for standard tier.
			sizeGB := 0
			if s, ok := snap.Properties["VolumeSize"].(int32); ok {
				sizeGB = int(s)
			} else if s, ok := snap.Properties["VolumeSize"].(int); ok {
				sizeGB = s
			}

			if sizeGB > 0 {
				snap.Cost = float64(sizeGB) * 0.05
			}
		}
	}

	return nil
}
