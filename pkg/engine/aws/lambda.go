package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/DrSkyle/cloudslash/pkg/graph"
)

type LambdaScanner struct {
	Client   *lambda.Client
	CWClient *cloudwatch.Client
	Graph    *graph.Graph
}

func NewLambdaScanner(cfg aws.Config, g *graph.Graph) *LambdaScanner {
	return &LambdaScanner{
		Client:   lambda.NewFromConfig(cfg),
		CWClient: cloudwatch.NewFromConfig(cfg),
		Graph:    g,
	}
}

// ScanFunctions discovers Lambda functions and usage metrics.
// Analyzes metrics over a 90-day window.
func (s *LambdaScanner) ScanFunctions(ctx context.Context) error {
	paginator := lambda.NewListFunctionsPaginator(s.Client, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, fn := range page.Functions {
			name := *fn.FunctionName
			arn := *fn.FunctionArn

			props := map[string]interface{}{
				"Service":      "Lambda",
				"Runtime":      string(fn.Runtime),
				"LastModified": *fn.LastModified, // Timestamp string.
				"CodeSize":     fn.CodeSize,
				"MemorySize":   fn.MemorySize,
			}

			s.Graph.AddNode(name, "aws_lambda_function", props)

			// Check for function staleness (code rot).
			go s.checkCodeRot(ctx, name, props)

			// Check for version accumulation.
			go s.scanVersionsAndAliases(ctx, name, arn)
		}
	}
	return nil
}

func (s *LambdaScanner) checkCodeRot(ctx context.Context, funcName string, props map[string]interface{}) {
	node := s.Graph.GetNode(funcName)
	exists := (node != nil)
	if !exists { return }

	endTime := time.Now()
	startTime := endTime.Add(-90 * 24 * time.Hour) // 90-day window.

	queries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m_invocations"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String("AWS/Lambda"),
					MetricName: aws.String("Invocations"),
					Dimensions: []cwtypes.Dimension{{Name: aws.String("FunctionName"), Value: aws.String(funcName)}},
				},
				Period: aws.Int32(86400),
				Stat:   aws.String("Sum"),
			},
		},
	}

	out, err := s.CWClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         &startTime,
		EndTime:           &endTime,
	})

	if err != nil { return }

	totalInvocations := 0.0
	for _, res := range out.MetricDataResults {
		for _, v := range res.Values {
			totalInvocations += v
		}
	}

	s.Graph.Mu.Lock()
	node.Properties["SumInvocations90d"] = totalInvocations
	s.Graph.Mu.Unlock()
}

func (s *LambdaScanner) scanVersionsAndAliases(ctx context.Context, funcName string, funcArn string) {
	// Retrieve aliases to whitelist active versions.
	aliases := make(map[string]bool) // Key: Version ID
	
	aPaginator := lambda.NewListAliasesPaginator(s.Client, &lambda.ListAliasesInput{FunctionName: aws.String(funcName)})
	for aPaginator.HasMorePages() {
		page, err := aPaginator.NextPage(ctx)
		if err == nil {
			for _, alias := range page.Aliases {
				aliases[*alias.FunctionVersion] = true
			}
		}
	}

	// Retrieve all function versions.
	vPaginator := lambda.NewListVersionsByFunctionPaginator(s.Client, &lambda.ListVersionsByFunctionInput{FunctionName: aws.String(funcName)})
	
	var versions []string
	var totalSize int64

	for vPaginator.HasMorePages() {
		page, err := vPaginator.NextPage(ctx)
		if err != nil { break }
		
		for _, v := range page.Versions {
			if *v.Version == "$LATEST" {
				continue // Exclude $LATEST alias.
			}
			versions = append(versions, *v.Version)
			totalSize += v.CodeSize
		}
	}


	node := s.Graph.GetNode(funcName)
	if node != nil {
		s.Graph.Mu.Lock()
		node.Properties["AllVersions"] = versions
		node.Properties["AliasVersions"] = aliases // map[string]bool
		node.Properties["TotalCodeSizeBytes"] = totalSize
		node.Properties["VersionCount"] = len(versions)
		s.Graph.Mu.Unlock()
	}
}
