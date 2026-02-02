package remediation

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/lazarus"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/storage"
)

// ExecuteSafeDeletion executes deletion.
func ExecuteSafeDeletion(ctx context.Context, node *graph.Node, deleter *aws.Deleter, store storage.BlobStore) error {
	resourceID := node.IDStr()

	// 1. Tombstone
	region := "unknown"
	if r, ok := node.Properties["Region"].(string); ok {
		region = r
	}

	ts := lazarus.NewTombstone(resourceID, node.TypeStr(), region, node.Properties)
	if err := ts.Save(ctx, store); err != nil {
		return fmt.Errorf("safety check failed: unable to save tombstone: %w", err)
	}

	// 2. Snapshot (if Volume)
	if node.TypeStr() == "AWS::EC2::Volume" {
		desc := fmt.Sprintf("CloudSlash Safety Backup for %s", resourceID)
		if _, err := deleter.CreateSnapshot(ctx, resourceID, desc); err != nil {
			return fmt.Errorf("safety check failed: unable to create snapshot: %w", err)
		}
	}

	// 3. Delete
	if node.TypeStr() == "AWS::EC2::Volume" {
		return deleter.DeleteVolume(ctx, resourceID)
	}

	return fmt.Errorf("unsupported resource type for deletion: %s", node.TypeStr())
}
