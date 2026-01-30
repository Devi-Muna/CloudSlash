package tetris

import (
	"math"
	"sort"
)

// Packer implements the bin packing strategy.
type Packer struct {
	availableBins []*Bin // Empty bins waiting to be used (e.g. from Solver catalog)
}

func NewPacker() *Packer {
	return &Packer{}
}

// Pack executes Best Fit Decreasing (BFD).
func (p *Packer) Pack(items []*Item, binFactory func() *Bin) []*Bin {
	// Sort items by size (descending).
	// Reduces fragmentation.
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

		// Find best fit bin.
		for i, bin := range usedBins {
			if canFit(bin, item) {
				// Calculate residual space.
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
			// Assign to bin.
			usedBins[bestFitIndex].AddItem(item)
		} else {
			// Allocate new bin.
			newBin := binFactory()
			if !newBin.AddItem(item) {
				// Item too large for optimal binning.
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
