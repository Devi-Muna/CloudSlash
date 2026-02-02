package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/audit"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/aws"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/heuristics"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/remediation"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/swarm"
	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/DrSkyle/cloudslash/v2/pkg/storage"
	"github.com/spf13/cobra"
)

var CleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Interactive resource remediation",
	Long:  `Iteratively reviews identified unused resources and performs real deletion with confirmation.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("[CRITICAL] DESTRUCTIVE OPERATION INITIATED")
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



		fmt.Println("\n[SCAN] Analyzing infrastructure topology...")
		client, err := aws.NewClient(ctx, config.Region, "", config.Verbose)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Initialize heuristics.
		hEngine := heuristics.NewEngine()
		hEngine.Register(&heuristics.UnattachedVolumeHeuristic{Pricing: nil})

		ec2 := aws.NewEC2Scanner(client.Config, g)
		ec2.ScanVolumes(ctx)
		hEngine.Run(ctx, g)

		// Filter for waste nodes.
		g.Mu.RLock()
		var waste []*graph.Node
		for _, node := range g.GetNodes() {
			if node.IsWaste {
				waste = append(waste, node)
			}
		}
		g.Mu.RUnlock()

		if len(waste) == 0 {
			fmt.Println("No unused resources detected. Infrastructure is optimized.")
			return
		}

		fmt.Printf("\nFound %d unused resources.\n", len(waste))

		deleter := aws.NewDeleter(client.Config)

		for _, item := range waste {
			fmt.Printf("\n[TARGET] %s (%s)\n", item.IDStr(), item.TypeStr())
			fmt.Printf(" Reason: %s\n", item.Properties["Reason"])

			fmt.Printf("\n[TARGET] %s (%s)\n", item.IDStr(), item.TypeStr())
			fmt.Printf(" Reason: %s\n", item.Properties["Reason"])

			// Check dependencies.
			dependents := g.GetUpstream(item.IDStr())
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
					fmt.Printf("\n [WARNING] DEPENDENCY IMPACT: Used by %d ACTIVE resources (e.g. %s)\n", activeDeps, exampleDep)
					fmt.Printf("    Deleting this may break them.\n")
				}
			}

			fmt.Print(" [ACTION] DELETE this resource? [y/N]: ")

			if scanner.Scan() {
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans == "y" {
					fmt.Printf("    [Safety] initializing Lazarus Protocol for %s...\n", item.IDStr())

					store := storage.NewLocalStore(".cloudslash/tombstones")

					// Execute safe deletion.
					err := remediation.ExecuteSafeDeletion(ctx, item, deleter, store)
					if err != nil {
						fmt.Printf("    [FATAL] Safety Check Failed: %v\n    ABORTING DELETION.\n", err)
						continue
					}

					fmt.Printf("    [Success] Resource securely remediated.\n")
					audit.LogAction("DELETED", item.IDStr(), item.TypeStr(), item.Cost, fmt.Sprintf("%v", item.Properties["Reason"]))
				} else {
					fmt.Println("    Skipped.")
				}
			}
		}

		fmt.Println("\nCleanup complete.")
	},
}
