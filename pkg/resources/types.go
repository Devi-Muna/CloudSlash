package resources

// AWS Resource Types
const (
	EC2Instance       = "AWS::EC2::Instance"
	EC2Volume         = "AWS::EC2::Volume"
	EC2VPC            = "AWS::EC2::VPC"
	EC2SecurityGroup  = "AWS::EC2::SecurityGroup"
	EC2Subnet         = "AWS::EC2::Subnet"
	EC2NatGateway     = "AWS::EC2::NatGateway"
	EC2EIP            = "AWS::EC2::EIP"
	EC2RouteTable     = "AWS::EC2::RouteTable"
	EC2InternetGateway = "AWS::EC2::InternetGateway"

	LambdaFunction    = "AWS::Lambda::Function"
	S3Bucket          = "AWS::S3::Bucket"
	RDSInstance       = "AWS::RDS::DBInstance"
	RDSSnapshot       = "AWS::RDS::DBSnapshot"
	DynamoDBTable     = "AWS::DynamoDB::Table"
	IAMRole           = "AWS::IAM::Role"
	IAMUser           = "AWS::IAM::User"
	ECRRepository     = "AWS::ECR::Repository"
	ECSCluster        = "AWS::ECS::Cluster"
	ECSService        = "AWS::ECS::Service"
	EBSSnapshot       = "AWS::EC2::Snapshot" // Assuming this naming convention from code
	LoadBalancer      = "AWS::ElasticLoadBalancingV2::LoadBalancer" // Check actual usage
)
