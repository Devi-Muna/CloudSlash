package remediation

import "testing"

func TestExtractResourceID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "EC2 Volume ARN",
			id:   "arn:aws:ec2:us-east-1:123456789012:volume/vol-0123456789abcdef0",
			want: "vol-0123456789abcdef0",
		},
		{
			name: "RDS Instance ARN",
			id:   "arn:aws:rds:us-east-1:123456789012:db:mysql-db-1",
			want: "mysql-db-1",
		},
		{
			name: "Simple ID",
			id:   "vol-0123456789abcdef0",
			want: "vol-0123456789abcdef0",
		},
		{
			name: "S3 Bucket ARN",
			id:   "arn:aws:s3:::my-bucket",
			want: "my-bucket",
		},
		{
			name: "IAM Role ARN",
			id:   "arn:aws:iam::123456789012:role/my-role",
			want: "my-role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractResourceID(tt.id); got != tt.want {
				t.Errorf("extractResourceID() = %v, want %v", got, tt.want)
			}
		})
	}
}
