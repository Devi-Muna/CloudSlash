package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/DrSkyle/cloudslash/internal/heuristics"
	"github.com/DrSkyle/cloudslash/internal/swarm"
	"github.com/spf13/cobra"
)

var NukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Interactive cleanup (The 'Safety Brake')",
	Long:  `Iteratively reviews identified waste and performs real deletion with confirmation.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  WARNING: You are entering DESTRUCTIVE MODE.")
		fmt.Println("   This will DELETE resources from your AWS account.")
		fmt.Print("   Are you sure? [y/N]: ")
		
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			text := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if text != "y" && text != "yes" {
				fmt.Println("Aborted.")
				return
			}
		}

		ctx := context.Background()
		g := graph.NewGraph()
		engine := swarm.NewEngine()
		engine.Start(ctx)

		// 1. Run a fresh scan (Headless)
		fmt.Println("\n🔍 Scanning infrastructure for targets...")
		// Note: We need to access internal scan logic.
		// Reusing app.Config and mimicking bootstrap logic quickly.
		// Ideally refactor bootstrap to return the graph.
		// For now, let's assume valid creds and basic scan.
		client, err := aws.NewClient(ctx, "us-east-1", "") // Default region/profile
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		
		// Setup minimal heuristics
		hEngine := heuristics.NewEngine()
		hEngine.Register(&heuristics.ZombieEBSHeuristic{Pricing: nil}) // Pricing optional for nuke
		// ... (Add others if needed, focusing on EBS for safety demo)

		// RUN SCAN (Simplified for Nuke - just EBS for now to prove concept safely)
		// Full scan is heavy. Let's just do EBS Nuke for v1.2
		ec2 := aws.NewEC2Scanner(client.Config, g)
		ec2.ScanVolumes(ctx)
		hEngine.Run(ctx, g)

		// 2. Iterate and Destroy
		g.Mu.RLock()
		var waste []*graph.Node
		for _, node := range g.Nodes {
			if node.IsWaste {
				waste = append(waste, node)
			}
		}
		g.Mu.RUnlock()

		if len(waste) == 0 {
			fmt.Println("No waste found to nuke. You are clean.")
			return
		}

		fmt.Printf("\nFound %d waste items.\n", len(waste))
		
		deleter := aws.NewDeleter(client.Config) // Need to implement this or just use client directly

		for _, item := range waste {
			fmt.Printf("\n[TARGET] %s (%s)\n", item.ID, item.Type)
			fmt.Printf(" Reason: %s\n", item.Properties["Reason"])
			fmt.Print(" 💀 Delete this resource? [y/N]: ")
			
			if scanner.Scan() {
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans == "y" {
					fmt.Printf("    Destroying %s... ", item.ID)
					// Verify implementation of Delete
					err := deleter.DeleteVolume(ctx, item.ID)
					if err != nil {
						fmt.Printf("FAILED: %v\n", err)
					} else {
						fmt.Printf("GONE.\n")
					}
				} else {
					fmt.Println("    Skipped.")
				}
			}
		}
		
		fmt.Println("\nNuke complete.")
	},
}
