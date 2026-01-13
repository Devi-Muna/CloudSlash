package config

import "time"

// HeuristicConfig holds all heuristic-related configurations.
type HeuristicConfig struct {
	IdleCluster IdleClusterConfig `mapstructure:"idle_cluster"`
	UnattachedVolume   UnattachedVolumeConfig   `mapstructure:"unattached_volume"`
	S3Multipart S3MultipartConfig `mapstructure:"s3_multipart"`
}

type IdleClusterConfig struct {
	CPUThreshold    float64       `mapstructure:"cpu_threshold"`
	UptimeThreshold time.Duration `mapstructure:"uptime_threshold"`
}

type UnattachedVolumeConfig struct {
	UnusedDays int      `mapstructure:"unused_days"`
	IgnoreTags []string `mapstructure:"ignore_tags"`
}

type S3MultipartConfig struct {
	AgeThreshold time.Duration `mapstructure:"age_threshold"`
}

// DefaultHeuristicConfig returns the safe defaults (current hardcoded values).
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
