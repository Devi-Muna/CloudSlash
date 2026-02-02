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

	// Scenario:
	// A: 950 CPU, 3900 RAM (Fits 2 per node with sidecar gap).
	// Node Capacity used: 2x950 = 1900 mCPU. Remaining: 100 mCPU.
	// Sidecar: 100 mCPU. Fits perfectly in the remainder.
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

	// Verification:
	// Bin 1: pod-1, pod-2, sidecar (100% CPU utilization).
	// Bin 2: pod-3, pod-4, sidecar (100% CPU utilization).

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
