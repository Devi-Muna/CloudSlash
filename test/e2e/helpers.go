//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetAWSConfig returns the shared AWS config pointing to LocalStack
func GetAWSConfig(t *testing.T) aws.Config {
	if awsCfg.Region == "" {
		t.Fatal("AWS Config not initialized (TestMain didn't run?)")
	}
	return awsCfg
}

// ProvisionEC2Instance creates a dummy instance in LocalStack
func ProvisionEC2Instance(t *testing.T, client *ec2.Client, tags map[string]string) string {
	t.Helper()

	var tagSpecs []types.Tag
	for k, v := range tags {
		tagSpecs = append(tagSpecs, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	runOut, err := client.RunInstances(context.TODO(), &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"), // LocalStack doesn't care about real AMI IDs usually
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         tagSpecs,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to provision EC2: %v", err)
	}

	id := *runOut.Instances[0].InstanceId
	t.Logf("Provisioned instance %s with tags %v", id, tags)
	return id
}

// GetInstanceState helper
func GetInstanceState(t *testing.T, client *ec2.Client, instanceID string) types.InstanceStateName {
	t.Helper()
	out, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		t.Fatalf("Failed to describe instance %s: %v", instanceID, err)
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		t.Fatalf("Instance %s not found", instanceID)
	}
	return out.Reservations[0].Instances[0].State.Name
}

// GetBinaryPath builds CLI and returns binary path
func GetBinaryPath(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "cloudslash")
	// Navigate to root
	rootDir := "../../" 
	cmd := exec.Command("go", "build", "-o", binPath, "cmd/cloudslash-cli/main.go")
	cmd.Dir = rootDir
	// Inherit env
	cmd.Env = os.Environ()
	
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %s", out)
	}
	return binPath
}
