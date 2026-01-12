package tetris

// Dimensions represents CPU and RAM.
// CPU is in millicores (1000m = 1 vCPU).
// RAM is in MiB.
type Dimensions struct {
	CPU float64
	RAM float64
}

// Item represents a workload (e.g., K8s Pod).
type Item struct {
	ID         string
	Dimensions Dimensions
	Group      string // e.g. "frontend", "backend" - for future Affinity rules
}

// Bin represents a compute node (e.g., EC2 Instance).
type Bin struct {
	ID       string
	Capacity Dimensions
	Items    []*Item
	Used     Dimensions
}

// AddItem attempts to place an item in the bin. Returns true if successful.
func (b *Bin) AddItem(item *Item) bool {
	if b.Used.CPU+item.Dimensions.CPU > b.Capacity.CPU {
		return false
	}
	if b.Used.RAM+item.Dimensions.RAM > b.Capacity.RAM {
		return false
	}

	b.Items = append(b.Items, item)
	b.Used.CPU += item.Dimensions.CPU
	b.Used.RAM += item.Dimensions.RAM
	return true
}

// Waste returns the unused dimensions.
func (b *Bin) Waste() Dimensions {
	return Dimensions{
		CPU: b.Capacity.CPU - b.Used.CPU,
		RAM: b.Capacity.RAM - b.Used.RAM,
	}
}

// Efficiency returns the packing density (0.0 - 1.0).
// Simpler average of CPU and RAM utilization.
func (b *Bin) Efficiency() float64 {
	cpuEff := b.Used.CPU / b.Capacity.CPU
	ramEff := b.Used.RAM / b.Capacity.RAM
	return (cpuEff + ramEff) / 2.0
}
