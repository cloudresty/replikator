package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Logging struct {
		MinimumLevel string
	}
	Watcher struct {
		TimeoutSeconds int
	}
	Kubernetes struct {
		SkipTlsVerify bool
	}
	Reflection struct {
		DeleteOrphanedMirrors bool
	}
}

func Load() *Config {
	cfg := &Config{}

	if v := os.Getenv("LOGGING_MINIMUM_LEVEL"); v != "" {
		cfg.Logging.MinimumLevel = v
	} else {
		cfg.Logging.MinimumLevel = "Information"
	}

	if v := os.Getenv("WATCHER_TIMEOUT_SECONDS"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil {
			cfg.Watcher.TimeoutSeconds = seconds
		}
	} else {
		cfg.Watcher.TimeoutSeconds = 3600
	}

	if v := os.Getenv("KUBERNETES_SKIP_TLS_VERIFY"); v == "true" {
		cfg.Kubernetes.SkipTlsVerify = true
	}

	if v := os.Getenv("REFLECTION_DELETE_ORPHANED_MIRRORS"); v == "true" {
		cfg.Reflection.DeleteOrphanedMirrors = true
	} else {
		cfg.Reflection.DeleteOrphanedMirrors = false
	}

	return cfg
}

func (c *Config) WatcherTimeout() time.Duration {
	return time.Duration(c.Watcher.TimeoutSeconds) * time.Second
}

type WatcherConfig struct {
	timeout    time.Duration
	timeoutSec int
}

func NewWatcherConfig(cfg *Config) *WatcherConfig {
	return &WatcherConfig{
		timeout:    cfg.WatcherTimeout(),
		timeoutSec: cfg.Watcher.TimeoutSeconds,
	}
}

func (w *WatcherConfig) Timeout() time.Duration {
	return w.timeout
}

func (w *WatcherConfig) ShouldRefresh() bool {
	return w.timeout > 0
}
