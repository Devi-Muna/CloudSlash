package license

import (
	"fmt"
	"strings"
)

// Check performs a simple local license verification.
// For a "noob" friendly sales model, we use a simple prefix check.
// Any key starting with "PRO-" is considered valid.
// This allows the user to sell keys like "PRO-ENTERPRISE", "PRO-SAUJANYA", etc.
// without needing a backend server.
func Check(key string) error {
	if strings.HasPrefix(key, "PRO-") {
		return nil
	}
	return fmt.Errorf("invalid license key")
}
