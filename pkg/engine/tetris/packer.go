package tetris

import (
	"math"
	"sort"
)

// Packer handles the logic of placing Items into Bins.
type Packer struct {
	availableBins []*Bin // Empty bins waiting to be used (e.g. from Solver catalog)
}

func NewPacker() *Packer {
	return &Packer{}
}

// Pack performs Best Fit Decreasing (BFD) 2D bin packing.
// It takes a list of items and a factory function to create new bins (nodes).
// It returns the list of used bins.
func (p *Packer) Pack(items []*Item, binFactory func() *Bin) []*Bin {
	// 1. Sort Items Decreasing (BFD Strategy).
	// Constraints are handled by sorting items by resource magnitude (CPU * RAM).
	// This ensures larger items are placed first, minimizing fragmentation.
	// This ensures larger items are placed first, minimizing fragmentation.
	sort.Slice(items, func(i, j int) bool {
		areaI := items[i].Dimensions.CPU * items[i].Dimensions.RAM
		areaJ := items[j].Dimensions.CPU * items[j].Dimensions.RAM
		return areaI > areaJ
	})

	var usedBins []*Bin

	for _, item := range items {
		bestFitIndex := -1
		minResidualSpace := math.MaxFloat64

		// 2. Best Fit: Find the bin where this item leaves the minimum residual space.
		for i, bin := range usedBins {
			if canFit(bin, item) {
				// Calculate Residual Space (Current Capacity - Used - Item).
				// Metric: Sum of residual CPU and RAM.
				resCPU := (bin.Capacity.CPU - bin.Used.CPU) - item.Dimensions.CPU
				resRAM := (bin.Capacity.RAM - bin.Used.RAM) - item.Dimensions.RAM
				residual := resCPU + resRAM // Simple sum

				if residual < minResidualSpace {
					minResidualSpace = residual
					bestFitIndex = i
				}
			}
		}

		if bestFitIndex != -1 {
			// Found a home
			usedBins[bestFitIndex].AddItem(item)
		} else {
			// 3. Open a new Bin
			newBin := binFactory()
			if !newBin.AddItem(item) {
				// Item fits neither in existing bins nor a new empty bin.
				// This indicates the item exceeds the capacity of the largest available node type.
				continue 
			}
			usedBins = append(usedBins, newBin)
		}
	}

	return usedBins
}

func canFit(b *Bin, i *Item) bool {
	return (b.Used.CPU+i.Dimensions.CPU <= b.Capacity.CPU) &&
		(b.Used.RAM+i.Dimensions.RAM <= b.Capacity.RAM)
}
