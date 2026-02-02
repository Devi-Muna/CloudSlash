package permissions

// Catalog defines the known mapping of Scanners to IAM Actions.
var Catalog = map[string][]string{
	"EC2": {
		"ec2:DescribeInstances",
		"ec2:DescribeVolumes",
		"ec2:DescribeNatGateways",
		"ec2:DescribeAddresses",
		"ec2:DescribeSnapshots",
		"ec2:DescribeImages",
		"ec2:DescribeVolumesModifications",
		"ec2:DescribeInstanceTypes",
		"ec2:DescribeSecurityGroups",
		"ec2:DescribeSubnets",
		"ec2:DescribeVpcs",
	},
	"S3": {
		"s3:ListAllMyBuckets",
		"s3:GetBucketLocation",
		"s3:GetBucketTagging",
		"s3:GetBucketVersioning",
		"s3:GetLifecycleConfiguration",
		"s3:ListBucket", // For determining size/object count
	},
	"IAM": {
		"iam:ListUsers",
		"iam:ListRoles",
		"iam:GetAccessKeyLastUsed",
		"iam:ListAccessKeys",
		"iam:GetUser",
		"iam:GetRole",
	},
	"RDS": {
		"rds:DescribeDBInstances",
		"rds:DescribeDBClusters",
		"rds:ListTagsForResource",
	},
	"EKS": {
		"eks:ListClusters",
		"eks:DescribeCluster",
		"eks:ListNodegroups",
		"eks:DescribeNodegroup",
		"eks:ListFargateProfiles",
		"eks:DescribeFargateProfile",
	},
	"Lambda": {
		"lambda:ListFunctions",
		"lambda:GetFunction", // For runtime details
		"lambda:ListTags",
	},
	"CloudWatch": {
		"cloudwatch:GetMetricData",
		"cloudwatch:ListMetrics",
	},
}

// CorePermissions returns the absolute minimum permissions needed for the engine to boot.
func CorePermissions() []string {
	return []string{
		"sts:GetCallerIdentity",
		"organizations:DescribeOrganization", // Optional, for multi-account context
	}
}
