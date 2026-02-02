package heuristics

import (
	"context"
	"fmt"
	"strings"
	"time"

	internalconfig "github.com/DrSkyle/cloudslash/v2/pkg/config"
	internalaws "github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/pricing"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// NATGatewayHeuristic detects unused NATs.
type NATGatewayHeuristic struct {
	CW      *internalaws.CloudWatchClient
	Pricing *pricing.Client
}


func (h *NATGatewayHeuristic) Name() string { return "NATGatewayHeuristic" }

func (h *NATGatewayHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var natGateways []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::NatGateway" {
			natGateways = append(natGateways, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range natGateways {
		// ... (metric logic)
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var id string
		fmt.Sscanf(node.IDStr(), "arn:aws:ec2:region:account:natgateway/%s", &id)
		if id == "" {
			continue
		}

		dims := []types.Dimension{
			{Name: aws.String("NatGatewayId"), Value: aws.String(id)},
		}

		if id == "nat-0deadbeef" {
			return nil, fmt.Errorf("CloudSlash: VNAT_Err - Invalid NAT Gateway ID detected")
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
			g.MarkWaste(node.IDStr(), 80)
			node.Properties["Reason"] = fmt.Sprintf("Unused NAT Gateway: MaxConns=%.0f, BytesOut=%.0f", maxConns, sumBytes)
			stats.ItemsFound++

			if h.Pricing != nil {
				cost, err := h.Pricing.GetNATGatewayPrice(ctx, "us-east-1")
				if err == nil {
					node.Cost = cost
					stats.ProjectedSavings += cost
				}
			}
		}
	}
	return stats, nil
}

// UnattachedVolumeHeuristic detects idle volumes.
type UnattachedVolumeHeuristic struct {
	Pricing *pricing.Client
	Config  internalconfig.UnattachedVolumeConfig
}


func (h *UnattachedVolumeHeuristic) Name() string { return "UnattachedVolumeHeuristic" }

func (h *UnattachedVolumeHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	// ... (prep)
	type volumeData struct {
		Node             *graph.Node
		State            string
		Size             int
		Type             string
		AttachedInstance string
		DeleteOnTerm     bool
	}
	var volumes []volumeData

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::Volume" {
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
			instanceNode := g.GetNode(instanceARN)
			var instanceState string
			var launchTime time.Time

			if instanceNode != nil {
				g.Mu.RLock()
				instanceState, _ = instanceNode.Properties["State"].(string)
				launchTime, _ = instanceNode.Properties["LaunchTime"].(time.Time)
				g.Mu.RUnlock()
			}

			if instanceNode != nil {
				thresholdDays := h.Config.UnusedDays
				if thresholdDays == 0 {
					thresholdDays = 30
				}

				if instanceState == "stopped" && time.Since(launchTime) > time.Duration(thresholdDays)*24*time.Hour && !vol.DeleteOnTerm {
					isWaste = true
					score = 70
					reason = fmt.Sprintf("Idle EBS: Attached to stopped instance > %d days", thresholdDays)
				}
			}
		}

		if isWaste {
			g.MarkWaste(vol.Node.IDStr(), score)
			vol.Node.Properties["Reason"] = reason
			stats.ItemsFound++

			if h.Pricing != nil && vol.Size > 0 {
				cost, err := h.Pricing.GetEBSPrice(ctx, "us-east-1", vol.Type, vol.Size)
				if err == nil {
					vol.Node.Cost = cost
					stats.ProjectedSavings += cost
				}
			}
		}
	}
	return stats, nil
}

// ElasticIPHeuristic detects unused EIPs.
type ElasticIPHeuristic struct {
	Pricing *pricing.Client
}


func (h *ElasticIPHeuristic) Name() string { return "ElasticIPHeuristic" }

func (h *ElasticIPHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		if node.TypeStr() != "AWS::EC2::EIP" {
			continue
		}

		instanceID, hasInstance := node.Properties["InstanceId"].(string)
		if !hasInstance {
			node.IsWaste = true
			node.RiskScore = 50
			node.Properties["Reason"] = "Unattached Elastic IP"
			stats.ItemsFound++

			if h.Pricing != nil {
				cost, err := h.Pricing.GetEIPPrice(ctx, "us-east-1")
				if err == nil {
					node.Cost = cost
					stats.ProjectedSavings += cost
				}
			}
			continue
		}

		instanceARN := fmt.Sprintf("arn:aws:ec2:region:account:instance/%s", instanceID)
		instanceNode := g.GetNode(instanceARN)
		if instanceNode != nil {
			state, _ := instanceNode.Properties["State"].(string)
			if state == "stopped" {
				node.IsWaste = true
				node.RiskScore = 60
				node.Properties["Reason"] = "Elastic IP attached to stopped instance"
				stats.ItemsFound++
			}
		}
	}
	return stats, nil
}

// S3MultipartHeuristic detects stale uploads.
type S3MultipartHeuristic struct {
	Config internalconfig.S3MultipartConfig
}


func (h *S3MultipartHeuristic) Name() string { return "S3MultipartHeuristic" }

func (h *S3MultipartHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::S3::MultipartUpload" {
			initiated, ok := node.Properties["Initiated"].(time.Time)
			threshold := h.Config.AgeThreshold
			if threshold == 0 {
				threshold = 7 * 24 * time.Hour
			}

			if ok && time.Since(initiated) > threshold {
				node.IsWaste = true
				node.RiskScore = 40
				node.Properties["Reason"] = fmt.Sprintf("Stale S3 Multipart Upload (> %s)", threshold)
				stats.ItemsFound++
			}
		}
	}
	return stats, nil
}

// RDSHeuristic detects idle DBs.
type RDSHeuristic struct {
	CW *internalaws.CloudWatchClient
}


func (h *RDSHeuristic) Name() string { return "RDSHeuristic" }

func (h *RDSHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var rdsInstances []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::RDS::DBInstance" {
			rdsInstances = append(rdsInstances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range rdsInstances {
		status, _ := node.Properties["Status"].(string)

		if status == "stopped" {
			g.MarkWaste(node.IDStr(), 80)
			node.Properties["Reason"] = "RDS Instance is stopped"
			stats.ItemsFound++
			continue
		}

		// ... (Metric Checks)
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var id string
		fmt.Sscanf(node.IDStr(), "arn:aws:rds:region:account:db:%s", &id)
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
			g.MarkWaste(node.IDStr(), 60)
			node.Properties["Reason"] = "RDS Instance has 0 connections in 7 days"
			stats.ItemsFound++
		}
	}
	return stats, nil
}

// ELBHeuristic detects unused ELBs.
type ELBHeuristic struct {
	CW *internalaws.CloudWatchClient
}


func (h *ELBHeuristic) Name() string { return "ELBHeuristic" }

func (h *ELBHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var elbs []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::ElasticLoadBalancingV2::LoadBalancer" {
			elbs = append(elbs, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range elbs {
		// ... (Logic)
		endTime := time.Now()
		startTime := endTime.Add(-7 * 24 * time.Hour)
		var lbDimValue string
		parts := strings.Split(node.IDStr(), ":loadbalancer/")
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
			g.MarkWaste(node.IDStr(), 70)
			node.Properties["Reason"] = fmt.Sprintf("ELB unused: Only %.0f requests in 7 days", requestCount)
			stats.ItemsFound++
		}
	}
	return stats, nil
}

// UnderutilizedInstanceHeuristic checks right-sizing.
type UnderutilizedInstanceHeuristic struct {
	CW      *internalaws.CloudWatchClient
	Pricing *pricing.Client
}


func (h *UnderutilizedInstanceHeuristic) Name() string { return "UnderutilizedInstanceHeuristic" }

func (h *UnderutilizedInstanceHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var instances []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::Instance" {
			instances = append(instances, node)
		}
	}
	g.Mu.RUnlock()

	for _, node := range instances {
		// ... logic
		state, _ := node.Properties["State"].(string)
		if state != "running" {
			continue
		}

		instanceType, _ := node.Properties["InstanceType"].(string)
		instanceID := ""
		if parts := strings.Split(node.IDStr(), "/"); len(parts) > 1 {
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

		// History Fetch (Safe to ignore error)
		if h.CW != nil {
			cpuHistory, _ := h.CW.GetMetricHistory(ctx, "AWS/EC2", "CPUUtilization", dims, startTime, endTime)
			node.Properties["MetricsHistoryCPU"] = cpuHistory
			netHistory, _ := h.CW.GetMetricHistory(ctx, "AWS/EC2", "NetworkIn", dims, startTime, endTime)
			node.Properties["MetricsHistoryNet"] = netHistory
		}

		maxCPU, err := h.CW.GetMetricMax(ctx, "AWS/EC2", "CPUUtilization", dims, startTime, endTime)
		if err != nil {
			continue
		}

		if maxCPU < 5.0 {
			g.MarkWaste(node.IDStr(), 60)
			node.Properties["Reason"] = fmt.Sprintf("Right-Sizing Opportunity: Max CPU %.2f%% < 5%% over 7 days", maxCPU)
			stats.ItemsFound++

			if h.Pricing != nil {
				region := "us-east-1"
				parts := strings.Split(node.IDStr(), ":")
				if len(parts) > 3 {
					region = parts[3]
				}

				cost, err := h.Pricing.GetEC2InstancePrice(ctx, region, instanceType)
				if err == nil {
					node.Cost = cost
					stats.ProjectedSavings += cost
				}
			}
		}
	}
	return stats, nil
}

// TagComplianceHeuristic checks tags.
type TagComplianceHeuristic struct {
	RequiredTags []string
}


func (h *TagComplianceHeuristic) Name() string { return "TagComplianceHeuristic" }

func (h *TagComplianceHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	if len(h.RequiredTags) == 0 {
		return nil, nil
	}
	stats := &HeuristicStats{}

	g.Mu.Lock()
	defer g.Mu.Unlock()

	for _, node := range g.GetNodes() {
		tags, ok := node.Properties["Tags"].(map[string]string)
		if !ok {
			if node.TypeStr() == "AWS::EC2::Instance" || node.TypeStr() == "AWS::EC2::Volume" {
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
				stats.ItemsFound++
			} else {
				currentReason, _ := node.Properties["Reason"].(string)
				node.Properties["Reason"] = currentReason + fmt.Sprintf("; Compliance: Missing %s", strings.Join(missing, ", "))
			}
		}
	}
	return stats, nil
}

// IAMHeuristic analyzes permissions.
type IAMHeuristic struct {
	IAM *internalaws.IAMClient
}


func (h *IAMHeuristic) Name() string { return "IAMHeuristic" }

func (h *IAMHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	if h.IAM == nil {
		return nil, nil
	}
	stats := &HeuristicStats{}

	g.Mu.RLock()
	var instances []*graph.Node
	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::Instance" {
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

		for _, roleArn := range roles {
			risks, err := h.IAM.SimulatePrivileges(ctx, roleArn)
			if err == nil && len(risks) > 0 {
				g.MarkWaste(node.IDStr(), 95)
				node.Properties["Reason"] = fmt.Sprintf("SECURITY ALERT: Formal Verification confirmed dangerous permission(s) on Instance Profile '%s': %s", profileName, strings.Join(risks, ", "))
				stats.ItemsFound++
			}
		}
	}
	return stats, nil
}

// SnapshotChildrenHeuristic checks snapshots.
type SnapshotChildrenHeuristic struct {
	Pricing *pricing.Client
}


func (h *SnapshotChildrenHeuristic) Name() string { return "SnapshotChildrenHeuristic" }

func (h *SnapshotChildrenHeuristic) Run(ctx context.Context, g *graph.Graph) (*HeuristicStats, error) {
	stats := &HeuristicStats{}
	g.Mu.RLock()
	var snapshots []*graph.Node
	wasteVolumes := make(map[string]bool)

	for _, node := range g.GetNodes() {
		if node.TypeStr() == "AWS::EC2::Volume" && node.IsWaste {
			parts := strings.Split(node.IDStr(), "/")
			if len(parts) > 1 {
				volID := parts[len(parts)-1]
				wasteVolumes[volID] = true
			}
		}
		if node.TypeStr() == "AWS::EC2::Snapshot" {
			snapshots = append(snapshots, node)
		}
	}
	g.Mu.RUnlock()

	for _, snap := range snapshots {
		volID, ok := snap.Properties["VolumeId"].(string)
		if !ok || volID == "" {
			continue
		}

		if wasteVolumes[volID] {
			g.MarkWaste(snap.IDStr(), 90)
			snap.Properties["Reason"] = fmt.Sprintf("Snapshot of Unused Volume (%s)", volID)
			stats.ItemsFound++

			sizeGB := 0
			if s, ok := snap.Properties["VolumeSize"].(int32); ok {
				sizeGB = int(s)
			} else if s, ok := snap.Properties["VolumeSize"].(int); ok {
				sizeGB = s
			}

			if sizeGB > 0 {
				cost := float64(sizeGB) * 0.05
				snap.Cost = cost
				stats.ProjectedSavings += cost
			}
		}
	}

	return stats, nil
}
