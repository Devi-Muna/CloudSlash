package tetris

import (
	"fmt"
	"testing"
)

func TestPacker_Tetris(t *testing.T) {
	// Standard AWS EC2 Sizes (ish)
	// m5.large: 2 vCPU, 8 GiB RAM
	binFactory := func() *Bin {
		return &Bin{
			Capacity: Dimensions{CPU: 2000, RAM: 8192},
		}
	}

	// 1. Create a set of Pods that SHOULD fit perfectly into 2 nodes mathematically.
	// Node Cap: 4000 CPU, 16384 RAM total (for 2 nodes)
	
	// A: 950 CPU, 3900 RAM (Fits 2 per node with gap for sidecar)
	// Node Cap: 2000. 2x950 = 1900. Gap = 100.
	// Sidecar: 100. Fits perfectly.
	items := []*Item{
		{ID: "pod-1", Dimensions: Dimensions{CPU: 950, RAM: 3900}},
		{ID: "pod-2", Dimensions: Dimensions{CPU: 950, RAM: 3900}},
		{ID: "pod-3", Dimensions: Dimensions{CPU: 950, RAM: 3900}},
		{ID: "pod-4", Dimensions: Dimensions{CPU: 950, RAM: 3900}},
		// B: Small fragmentation fillers
		{ID: "sidecar-1", Dimensions: Dimensions{CPU: 100, RAM: 100}},
		{ID: "sidecar-2", Dimensions: Dimensions{CPU: 100, RAM: 100}},
	}

	packer := NewPacker()
	bins := packer.Pack(items, binFactory)

	// Expectation:
	// pod-1, pod-2 fit in Bin 1 (Remaining: 0 CPU, 192 RAM)
	// pod-3, pod-4 fit in Bin 2
	// Sidecars fit in gaps logic.
	
	fmt.Printf("\n🧩 Tetris Packing Result: %d Bins Used\n", len(bins))
	for i, b := range bins {
		fmt.Printf(" [ Bin %d ] Efficiency: %.1f%%\n", i+1, b.Efficiency()*100)
		for _, item := range b.Items {
			fmt.Printf("   ├─ %s (%.0f mCPU / %.0f MiB)\n", item.ID, item.Dimensions.CPU, item.Dimensions.RAM)
		}
		waste := b.Waste()
		fmt.Printf("   └─ Waste: %.0f mCPU / %.0f MiB\n", waste.CPU, waste.RAM)
	}

	if len(bins) > 2 {
		t.Errorf("Inefficient Packing! Used %d bins, expected 2.", len(bins))
	}
}
