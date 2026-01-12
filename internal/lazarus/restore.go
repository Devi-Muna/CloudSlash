package lazarus

import (
	"fmt"
	"strings"
)

// InstructionGenerator handles the creation of Terraform import commands
type InstructionGenerator struct{}

// GenerateImportCommand creates the command a user must run to drag a restored resource back into state.
// address: The Terraform address (e.g. module.vpc.aws_nat_gateway.main)
// newResourceID: The ID of the physically restored/existing resource (e.g. nat-0987654321)
func GenerateImportCommand(address string, newResourceID string) string {
	return fmt.Sprintf("terraform import %s %s", address, newResourceID)
}

// GenerateRestorationMessage creates the user-facing message for the "Drift-Based Restoration" workflow.
func GenerateRestorationMessage(resourceType, oldID, newID, tfAddress string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n[Lazarus Protocol] Resource Restored Successfully.\n"))
	sb.WriteString("-------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("Type:        %s\n", resourceType))
	sb.WriteString(fmt.Sprintf("Original ID: %s\n", oldID))
	sb.WriteString(fmt.Sprintf("New ID:      %s (Physically Active)\n", newID))
	sb.WriteString("-------------------------------------------------------------\n")
	sb.WriteString("ACTION REQUIRED: Synchronization\n")
	sb.WriteString("The resource exists in AWS but is disconnected from Terraform state.\n")
	sb.WriteString("To fix this 'Drift', run the following command in your terminal:\n\n")
	
	cmd := GenerateImportCommand(tfAddress, newID)
	sb.WriteString(fmt.Sprintf("  %s\n\n", cmd))
	
	sb.WriteString("Why? This avoids state corruption by forcing a clean import.\n")
	
	return sb.String()
}
