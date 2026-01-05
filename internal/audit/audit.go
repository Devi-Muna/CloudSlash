package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func LogAction(action, resourceID, resourceType string, cost float64, reason string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	
	logDir := filepath.Join(home, ".cloudslash")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}
	
	f, err := os.OpenFile(filepath.Join(logDir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	
	// Format: [DATE] Deleted nat-0x123 ($45/mo) - Reason: ...
	entry := fmt.Sprintf("[%s] %s %s (%s) - Savings: $%.2f/mo - Reason: %s\n", 
		time.Now().Format(time.RFC3339),
		action,
		resourceID,
		resourceType,
		cost,
		reason,
	)
	
	if _, err := f.WriteString(entry); err != nil {
		fmt.Printf("(Warning: Failed to write audit log)\n")
	}
}
