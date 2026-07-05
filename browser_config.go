package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ViewportSize holds browser viewport dimensions.
type ViewportSize struct {
	Width  int `json:"width"  yaml:"width"`
	Height int `json:"height" yaml:"height"`
}

// BrowserConfig holds all configurable browser options.
// Fields that affect anti-bot detection are marked accordingly.
type BrowserConfig struct {
	Browser           string            `json:"browser"             yaml:"browser"`             // firefox | chromium | webkit
	BrowserChannel    string            `json:"browser_channel"     yaml:"browser_channel"`     // e.g. "chrome", "msedge" (chromium only)
	UserAgent         string            `json:"user_agent"          yaml:"user_agent"`          // high impact
	Locale            string            `json:"locale"              yaml:"locale"`              // high impact: navigator.language
	TimezoneId        string            `json:"timezone_id"         yaml:"timezone_id"`         // high impact
	Viewport          *ViewportSize     `json:"viewport"            yaml:"viewport"`            // high impact
	DeviceScaleFactor float64           `json:"device_scale_factor" yaml:"device_scale_factor"` // high impact
	HasTouch          bool              `json:"has_touch"           yaml:"has_touch"`           // medium impact
	ColorScheme       string            `json:"color_scheme"        yaml:"color_scheme"`        // medium: light|dark|no-preference
	Permissions       []string          `json:"permissions"         yaml:"permissions"`
	ExtraHttpHeaders  map[string]string `json:"extra_http_headers"  yaml:"extra_http_headers"`
}

// defaultBrowserConfig returns the defaults matching the original hardcoded behaviour.
func defaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		Browser:     "firefox",
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Permissions: []string{"geolocation", "notifications"},
	}
}

// loadConfig reads a JSON or YAML config file and returns a BrowserConfig.
// Zero-value fields are filled with defaults afterwards by the caller.
func loadConfig(filePath string) (BrowserConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return BrowserConfig{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg BrowserConfig
	switch {
	case strings.HasSuffix(filePath, ".json"):
		err = json.Unmarshal(data, &cfg)
	case strings.HasSuffix(filePath, ".yaml"), strings.HasSuffix(filePath, ".yml"):
		err = yaml.Unmarshal(data, &cfg)
	default:
		return BrowserConfig{}, fmt.Errorf("unsupported config format (use .json, .yaml, or .yml)")
	}
	if err != nil {
		return BrowserConfig{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// applyDefaults fills zero-value fields in cfg with values from defaults.
func applyDefaults(cfg *BrowserConfig, defaults BrowserConfig) {
	if cfg.Browser == "" {
		cfg.Browser = defaults.Browser
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaults.UserAgent
	}
	if cfg.Permissions == nil {
		cfg.Permissions = defaults.Permissions
	}
}
