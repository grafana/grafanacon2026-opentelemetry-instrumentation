package chaos

import "os"

// Enabled returns true when CHAOS_MODE is set to "true" or "1".
func Enabled() bool {
	v := os.Getenv("CHAOS_MODE")
	return v == "true" || v == "1"
}
