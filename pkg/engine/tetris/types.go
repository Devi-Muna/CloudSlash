package tetris

// Dimensions defines compute resources.
// Dimensions defines compute resources.
// CPU in millicores, RAM in MiB.
type Dimensions struct {
	CPU float64
	RAM float64
}

// Item represents a deployable workload.
type Item struct {
	ID         string
	Dimensions Dimensions
	Group      string
}

// Bin represents a resource container.
type Bin struct {
	ID       string
	Capacity Dimensions
	Items    []*Item
	Used     Dimensions
}

// AddItem places an item inside the bin.
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

// Waste calculates unused capacity.
func (b *Bin) Waste() Dimensions {
	return Dimensions{
		CPU: b.Capacity.CPU - b.Used.CPU,
		RAM: b.Capacity.RAM - b.Used.RAM,
	}
}

// Efficiency calculates resource utilization.
func (b *Bin) Efficiency() float64 {
	cpuEff := b.Used.CPU / b.Capacity.CPU
	ramEff := b.Used.RAM / b.Capacity.RAM
	return (cpuEff + ramEff) / 2.0
}
