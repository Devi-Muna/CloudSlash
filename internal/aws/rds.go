package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type RDSScanner struct {
	Client *rds.Client
	Graph  *graph.Graph
}

func NewRDSScanner(cfg aws.Config, g *graph.Graph) *RDSScanner {
	return &RDSScanner{
		Client: rds.NewFromConfig(cfg),
		Graph:  g,
	}
}

func (s *RDSScanner) ScanInstances(ctx context.Context) error {
	paginator := rds.NewDescribeDBInstancesPaginator(s.Client, &rds.DescribeDBInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to describe rds instances: %v", err)
		}

		for _, instance := range page.DBInstances {
			// id := *instance.DBInstanceIdentifier // Unused
			arn := *instance.DBInstanceArn

			props := map[string]interface{}{
				"Status":        *instance.DBInstanceStatus,
				"InstanceClass": *instance.DBInstanceClass,
				"Engine":        *instance.Engine,
			}

			s.Graph.AddNode(arn, "AWS::RDS::DBInstance", props)
		}
	}
	return nil
}
