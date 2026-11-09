package config

const (
	RunLive            = "live"
	RunIsolatedRestore = "isolated_restore"
	RunMaintenance     = "maintenance"
)

// Empty retains the live behavior of callers that construct Config directly.
func (c Config) OutboundEnabled() bool { return c.RunMode == "" || c.RunMode == RunLive }
