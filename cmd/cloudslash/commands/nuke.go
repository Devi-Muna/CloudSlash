package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/aws"
	"github.com/DrSkyle/cloudslash/internal/audit"
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
		fmt.Println("[WARNING] DESTRUCTIVE MODE INITIATED")
		fmt.Println("This operation will permanently DELETE resources from your AWS account.")
		fmt.Print("Confirm execution? [y/N]: ")

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
		fmt.Println("\n[SCAN] Analyzing infrastructure topology...")
		// Note: We need to access internal scan logic.
		// Reusing app.Config and mimicking bootstrap logic quickly.
		// Ideally refactor bootstrap to return the graph.
		// For now, let's assume valid creds and basic scan.
		client, err := aws.NewClient(ctx, config.Region, "", config.Verbose) // Default region/profile
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
			fmt.Println("No waste detected. Infrastructure is optimized.")
			return
		}

		fmt.Printf("\nFound %d waste items.\n", len(waste))

		deleter := aws.NewDeleter(client.Config) // Need to implement this or just use client directly

		for _, item := range waste {
			fmt.Printf("\n[TARGET] %s (%s)\n", item.ID, item.Type)
			fmt.Printf(" Reason: %s\n", item.Properties["Reason"])

			// BLAST RADIUS CHECK
			dependents := g.GetUpstream(item.ID)
			if len(dependents) > 0 {
				activeDeps := 0
				exampleDep := ""
				g.Mu.RLock()
				for _, depID := range dependents {
					if depNode := g.GetNode(depID); depNode != nil {
						if !depNode.IsWaste {
							activeDeps++
							exampleDep = depID
						}
					}
				}
				g.Mu.RUnlock()

				if activeDeps > 0 {
					fmt.Printf("\n ⚠️  BLAST RADIUS WARNING: Used by %d ACTIVE resources (e.g. %s)\n", activeDeps, exampleDep)
					fmt.Printf("    Deleting this may break them.\n")
				}
			}

			fmt.Print(" [ACTION] DELETE this resource? [y/N]: ")

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
						audit.LogAction("DELETED", item.ID, item.Type, item.Cost, fmt.Sprintf("%v", item.Properties["Reason"]))
					}
				} else {
					fmt.Println("    Skipped.")
				}
			}
		}

		fmt.Println("\nNuke complete.")
	},
}
