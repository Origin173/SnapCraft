package config

// ApplyDefaultsForTest exposes defaults for unit tests.
func ApplyDefaultsForTest(cfg *Config) {
	applyDefaults(cfg)
}
