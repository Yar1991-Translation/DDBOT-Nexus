package roblox

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sora233/MiraiGo-Template/config"
)

const (
	defaultEnabled          = true
	defaultInterval         = 10 * time.Second
	defaultTimeout          = 8 * time.Second
	defaultBatchSize        = 100
	defaultUserAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	defaultJoinAPIEnabled   = true
	defaultEmitLeave        = true
	defaultUsernameCacheTTL = 24 * time.Hour
)

type ProviderConfig struct {
	Enabled          bool
	Interval         time.Duration
	Timeout          time.Duration
	BatchSize        int
	UserAgent        string
	JoinAPIEnabled   bool
	Roblosecurity    string
	EmitLeave        bool
	UsernameCacheTTL time.Duration
}

func loadProviderConfig() ProviderConfig {
	cfg := ProviderConfig{
		Enabled:          getBoolWithDefault("providers.roblox.enabled", defaultEnabled),
		Interval:         getDurationWithDefault("providers.roblox.interval", defaultInterval),
		Timeout:          getDurationWithDefault("providers.roblox.timeout", defaultTimeout),
		BatchSize:        getIntWithDefault("providers.roblox.batchSize", defaultBatchSize),
		UserAgent:        getStringWithDefault("providers.roblox.userAgent", defaultUserAgent),
		JoinAPIEnabled:   getBoolWithDefault("providers.roblox.joinApiEnabled", defaultJoinAPIEnabled),
		Roblosecurity:    strings.TrimSpace(getStringWithDefault("providers.roblox.roblosecurity", "")),
		EmitLeave:        getBoolWithDefault("providers.roblox.emitLeave", defaultEmitLeave),
		UsernameCacheTTL: getDurationWithDefault("providers.roblox.usernameCacheTTL", defaultUsernameCacheTTL),
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > 100 {
		cfg.BatchSize = 100
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.UsernameCacheTTL <= 0 {
		cfg.UsernameCacheTTL = defaultUsernameCacheTTL
	}
	return cfg
}

func (c ProviderConfig) maskedCookie() string {
	if c.Roblosecurity == "" {
		return "unset"
	}
	return fmt.Sprintf("set(len=%d)", len(c.Roblosecurity))
}

func (c ProviderConfig) canUseJoinAPI() bool {
	return c.JoinAPIEnabled && c.Roblosecurity != ""
}

func getBoolWithDefault(key string, fallback bool) bool {
	if config.GlobalConfig != nil && config.GlobalConfig.IsSet(key) {
		return config.GlobalConfig.GetBool(key)
	}
	return fallback
}

func getDurationWithDefault(key string, fallback time.Duration) time.Duration {
	if config.GlobalConfig != nil && config.GlobalConfig.IsSet(key) {
		d := config.GlobalConfig.GetDuration(key)
		if d > 0 {
			return d
		}
	}
	return fallback
}

func getIntWithDefault(key string, fallback int) int {
	if config.GlobalConfig != nil && config.GlobalConfig.IsSet(key) {
		val := config.GlobalConfig.GetInt(key)
		if val > 0 {
			return val
		}
	}
	return fallback
}

func getStringWithDefault(key string, fallback string) string {
	if config.GlobalConfig != nil && config.GlobalConfig.IsSet(key) {
		val := strings.TrimSpace(config.GlobalConfig.GetString(key))
		if val != "" {
			return val
		}
	}
	return fallback
}
