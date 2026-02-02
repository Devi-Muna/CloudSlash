package aws

import (
	"context"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
)

// Wrapper types adapting specific scanner methods to the generic scanner.Scanner interface.

// EC2InstanceScanner implements Scanner for ScanInstances.
type EC2InstanceScanner struct {
	Scanner *EC2Scanner
}

func (s *EC2InstanceScanner) Name() string { return "ScanInstances" }
func (s *EC2InstanceScanner) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanInstances(ctx)
}

// EC2VolumeScanner implements Scanner for ScanVolumes.
type EC2VolumeScanner struct {
	Scanner *EC2Scanner
}

func (s *EC2VolumeScanner) Name() string { return "ScanVolumes" }
func (s *EC2VolumeScanner) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanVolumes(ctx)
}

// EC2SnapshotScanner implements Scanner for ScanSnapshots.
type EC2SnapshotScanner struct {
	Scanner *EC2Scanner
	OwnerID string
}

func (s *EC2SnapshotScanner) Name() string { return "ScanSnapshots" }
func (s *EC2SnapshotScanner) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanSnapshots(ctx, s.OwnerID)
}

// EC2ImageScanner implements Scanner for ScanImages.
type EC2ImageScanner struct {
	Scanner *EC2Scanner
}

func (s *EC2ImageScanner) Name() string { return "ScanImages" }
func (s *EC2ImageScanner) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanImages(ctx)
}

// NATScannerWrapper implements Scanner for ScanNATGateways.
type NATScannerWrapper struct {
	Scanner *NATScanner
}

func (s *NATScannerWrapper) Name() string { return "ScanNATGateways" }
func (s *NATScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanNATGateways(ctx)
}

// EIPScannerWrapper implements Scanner for ScanAddresses.
type EIPScannerWrapper struct {
	Scanner *EIPScanner
}

func (s *EIPScannerWrapper) Name() string { return "ScanAddresses" }
func (s *EIPScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanAddresses(ctx)
}

// ALBScannerWrapper implements Scanner for ScanALBs.
type ALBScannerWrapper struct {
	Scanner *ALBScanner
}

func (s *ALBScannerWrapper) Name() string { return "ScanALBs" }
func (s *ALBScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanALBs(ctx)
}

// VPCEndpointScannerWrapper implements Scanner for ScanEndpoints.
type VPCEndpointScannerWrapper struct {
	Scanner *VpcEndpointScanner
}

func (s *VPCEndpointScannerWrapper) Name() string { return "ScanEndpoints" }
func (s *VPCEndpointScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanEndpoints(ctx)
}

// S3ScannerWrapper implements Scanner for ScanBuckets.
type S3ScannerWrapper struct {
	Scanner *S3Scanner
}

func (s *S3ScannerWrapper) Name() string { return "ScanBuckets" }
func (s *S3ScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanBuckets(ctx)
}

// RDSScannerWrapper implements Scanner for ScanInstances.
type RDSScannerWrapper struct {
	Scanner *RDSScanner
}

func (s *RDSScannerWrapper) Name() string { return "ScanRDSInstances" }
func (s *RDSScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanInstances(ctx)
}

// EKSScannerWrapper implements Scanner for ScanClusters.
type EKSScannerWrapper struct {
	Scanner *EKSScanner
}

func (s *EKSScannerWrapper) Name() string { return "ScanEKSClusters" }
func (s *EKSScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanClusters(ctx)
}

// ECSScannerWrapper implements Scanner for ScanClusters.
type ECSScannerWrapper struct {
	Scanner *ECSScanner
}

func (s *ECSScannerWrapper) Name() string { return "ScanECSClusters" }
func (s *ECSScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanClusters(ctx)
}

// ElasticacheScannerWrapper implements Scanner for ScanClusters.
type ElasticacheScannerWrapper struct {
	Scanner *ElasticacheScanner
}

func (s *ElasticacheScannerWrapper) Name() string { return "ScanElasticacheClusters" }
func (s *ElasticacheScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanClusters(ctx)
}

// RedshiftScannerWrapper implements Scanner for ScanClusters.
type RedshiftScannerWrapper struct {
	Scanner *RedshiftScanner
}

func (s *RedshiftScannerWrapper) Name() string { return "ScanRedshiftClusters" }
func (s *RedshiftScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanClusters(ctx)
}

// DynamoDBScannerWrapper implements Scanner for ScanTables.
type DynamoDBScannerWrapper struct {
	Scanner *DynamoDBScanner
}

func (s *DynamoDBScannerWrapper) Name() string { return "ScanDynamoDBTables" }
func (s *DynamoDBScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanTables(ctx)
}

// LambdaScannerWrapper implements Scanner for ScanFunctions.
type LambdaScannerWrapper struct {
	Scanner *LambdaScanner
}

func (s *LambdaScannerWrapper) Name() string { return "ScanLambdaFunctions" }
func (s *LambdaScannerWrapper) Scan(ctx context.Context, g *graph.Graph) error {
	return s.Scanner.ScanFunctions(ctx)
}
