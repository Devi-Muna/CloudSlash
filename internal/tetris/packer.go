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
	// 1. Sort Items Decreasing (Logic: Hardest constraints first)
	// We sort by "Area" (CPU * RAM) or Max Dimension?
	// Standard heuristic: Sort by biggest single resource demand?
	// Let's use scalar "Dominant Resource" heuristic.
	sort.Slice(items, func(i, j int) bool {
		// Calculate a "magnitude"
		// magI := items[i].Dimensions.CPU + items[i].Dimensions.RAM/1024 
		// Actually, let's just stick to "Area" proxy for now.
		areaI := items[i].Dimensions.CPU * items[i].Dimensions.RAM
		areaJ := items[j].Dimensions.CPU * items[j].Dimensions.RAM
		return areaI > areaJ
	})

	var usedBins []*Bin

	for _, item := range items {
		bestFitIndex := -1
		minResidualSpace := math.MaxFloat64

		// 2. Try to fit in existing used bins (Best Fit)
		for i, bin := range usedBins {
			if canFit(bin, item) {
				// Calculate "Residual Space" (Penalty)
				// We want the bin where this item leaves the LEAST room (tightest fit).
				// Heuristic: Residual Capacity Norm
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
				// Item is too big for empty bin! (Solver error usually)
				// For now, skip or panic? Let's skip and log.
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
