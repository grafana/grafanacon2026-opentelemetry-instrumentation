package chaos

import (
	"math/rand/v2"
	"os"
)

// Enabled returns true when CHAOS_MODE is set to "true" or "1".
func Enabled() bool {
	v := os.Getenv("CHAOS_MODE")
	return v == "true" || v == "1"
}

// Triggered returns true ~10% of the time when chaos mode is enabled.
func Triggered() bool {
	return Enabled() && rand.Float64() < 0.10
}
