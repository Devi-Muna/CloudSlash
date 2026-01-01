package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LicenseServerURL is the endpoint of your Cloudflare Worker.
// TODO: Update this to your deployed worker URL.
const LicenseServerURL = "https://cloudslash-license-server.nexus-apis.workers.dev/verify"

type VerifyRequest struct {
	LicenseKey string `json:"licenseKey"`
}

type VerifyResponse struct {
	Valid  bool       `json:"valid"`
	Plan   string     `json:"plan"`
	Expiry *time.Time `json:"expiry"` // Pointer to handle null/nil
	Reason string     `json:"reason"`
}

// Check validates the license key by calling the Cloudflare Worker (Freemius Proxy).
func Check(key string) error {
	// 1. Prepare Request
	reqBody, err := json.Marshal(VerifyRequest{LicenseKey: key})
	if err != nil {
		return fmt.Errorf("internal error: %v", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 2. Call API
	resp, err := client.Post(LicenseServerURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("license check failed: unable to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}

	// 3. Parse Response
	var result VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid server response")
	}

	if !result.Valid {
		return fmt.Errorf("license invalid: %s", result.Reason)
	}

	// Success!
	return nil
}
