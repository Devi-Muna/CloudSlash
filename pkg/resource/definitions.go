package resource

import "time"

// Resource represents a strictly typed infrastructure node.
type Resource interface {
	GetID() string
	GetType() string
}

// BaseResource implements common fields.
type BaseResource struct {
	ID     string
	Type   string
	Region string
	Tags   map[string]string
}

func (b *BaseResource) GetID() string {
	return b.ID
}

func (b *BaseResource) GetType() string {
	return b.Type
}

// EC2Instance represents an AWS EC2 Instance.
type EC2Instance struct {
	BaseResource
	State        string
	InstanceType string
	LaunchTime   time.Time
	VpcID        string
	SubnetID     string
	ImageID      string // AMI
}
