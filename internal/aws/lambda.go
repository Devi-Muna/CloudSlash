package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/DrSkyle/cloudslash/internal/graph"
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

// ScanFunctions handles Lambda analysis (Stale Code + Version Accumulation).
// Window: 90 Days.
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
				"LastModified": *fn.LastModified, // "2023-01-01T..." string
				"CodeSize":     fn.CodeSize,
				"MemorySize":   fn.MemorySize,
			}

			s.Graph.AddNode(name, "aws_lambda_function", props)
			// Graph ID = Name, ID in ARN sense = ARN. Should store ARN property or ID?
			// Using Name as ID for TUI readability.
			
			// 1. Check for Stale Functions (Invocations)
			go s.checkCodeRot(ctx, name, props)

			// 2. Check for Version Accumulation (Pruner)
			go s.scanVersionsAndAliases(ctx, name, arn)
		}
	}
	return nil
}

func (s *LambdaScanner) checkCodeRot(ctx context.Context, funcName string, props map[string]interface{}) {
	node := s.Graph.GetNode(funcName)
	// s.Graph.Mu.Unlock() - Removed, GetNode handles lock
	exists := (node != nil)
	if !exists { return }

	endTime := time.Now()
	startTime := endTime.Add(-90 * 24 * time.Hour) // 90 Days

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
	// 1. Get Aliases (Whitelist)
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

	// 2. Get Versions (Pagination Critical)
	vPaginator := lambda.NewListVersionsByFunctionPaginator(s.Client, &lambda.ListVersionsByFunctionInput{FunctionName: aws.String(funcName)})
	
	var versions []string
	var totalSize int64

	for vPaginator.HasMorePages() {
		page, err := vPaginator.NextPage(ctx)
		if err != nil { break }
		
		for _, v := range page.Versions {
			if *v.Version == "$LATEST" {
				continue // Always keep latest
			}
			versions = append(versions, *v.Version)
			totalSize += v.CodeSize
		}
	}

	// Store data for Heuristic to process (separation of concerns)
	// Store data for Heuristic to process (separation of concerns)
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
