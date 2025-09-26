package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config models the YAML configuration file that drives the monitor.
type Config struct {
	RPCURL        string        `yaml:"rpc_url"`
	PollInterval  string        `yaml:"poll_interval"`
	Assets        []AssetConfig `yaml:"assets"`
	Notifications Notifications `yaml:"notifications"`
}

// AssetConfig describes a single aToken that should be monitored.
type AssetConfig struct {
	Name             string `yaml:"name"`
	Address          string `yaml:"address"`
	TargetCapTokens  string `yaml:"target_cap_tokens"`
	NotifyOnIncrease *bool  `yaml:"notify_on_increase"`
	NotifyOnDecrease *bool  `yaml:"notify_on_decrease"`
	PollInterval     string `yaml:"poll_interval"`
}

// Notifications holds optional downstream integrations.
type Notifications struct {
	Telegram *TelegramConfig `yaml:"telegram"`
	JSONRPC  *JSONRPCConfig  `yaml:"json_rpc"`
}

// TelegramConfig configures Telegram bot notifications.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// JSONRPCConfig configures a custom JSON-RPC callback.
type JSONRPCConfig struct {
	URL string `yaml:"url"`
}

// Load reads and parses the YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.RPCURL == "" {
		return nil, errors.New("rpc_url must be provided")
	}

	if len(cfg.Assets) == 0 {
		return nil, errors.New("at least one asset must be configured")
	}

	return &cfg, nil
}
