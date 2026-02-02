package config

import "time"

// HeuristicConfig defines settings for resource analysis heuristics.
type HeuristicConfig struct {
	IdleCluster      IdleClusterConfig      `mapstructure:"idle_cluster"`
	UnattachedVolume UnattachedVolumeConfig `mapstructure:"unattached_volume"`
	S3Multipart      S3MultipartConfig      `mapstructure:"s3_multipart"`
}

type IdleClusterConfig struct {
	// CPUThreshold is the utilization percentage for idle detection.
	CPUThreshold float64 `mapstructure:"cpu_threshold"`
	// UptimeThreshold is the required duration below CPU threshold.
	UptimeThreshold time.Duration `mapstructure:"uptime_threshold"`
}

type UnattachedVolumeConfig struct {
	// UnusedDays is the number of days a volume must be unattached.
	UnusedDays int `mapstructure:"unused_days"`
	// IgnoreTags is a list of tag keys to ignore.
	IgnoreTags []string `mapstructure:"ignore_tags"`
}

type S3MultipartConfig struct {
	// AgeThreshold is the maximum age for incomplete multipart uploads.
	AgeThreshold time.Duration `mapstructure:"age_threshold"`
}

// DefaultHeuristicConfig returns a configuration with sensible default values.
func DefaultHeuristicConfig() HeuristicConfig {
	return HeuristicConfig{
		IdleCluster: IdleClusterConfig{
			CPUThreshold:    5.0,
			UptimeThreshold: 1 * time.Hour,
		},
		UnattachedVolume: UnattachedVolumeConfig{
			UnusedDays: 30,
			IgnoreTags: []string{},
		},
		S3Multipart: S3MultipartConfig{
			AgeThreshold: 7 * 24 * time.Hour, // 7 days
		},
	}
}
