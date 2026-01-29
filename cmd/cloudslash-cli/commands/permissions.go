package commands

import (
	"fmt"
	"os"

	"github.com/DrSkyle/cloudslash/pkg/engine/permissions"
	"github.com/spf13/cobra"
)

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Generate Least-Privilege IAM Policy",
	Long:  `Generates the exact AWS IAM JSON Policy required to run CloudSlash.`,
	Run: func(cmd *cobra.Command, args []string) {
		// In the future, we can filter by flags (e.g. --only ec2,s3)
		// For now, we generate the master policy.
		jsonBytes, err := permissions.GeneratePolicy(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating policy: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(jsonBytes))
	},
}

func init() {
	rootCmd.AddCommand(permissionsCmd)
}
