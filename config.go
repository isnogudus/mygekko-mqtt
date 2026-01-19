package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	LogLevel string       `toml:"log_level"`
	MyGekko  MyGekkoConfig `toml:"mygekko"`
	MQTT     MQTTConfig    `toml:"mqtt"`
}

type MyGekkoConfig struct {
	Host           string   `toml:"host"`
	Username       string   `toml:"username"`
	Password       string   `toml:"password"`
	Interval       float64  `toml:"interval"`
	IntervalItems  []string `toml:"interval_items"`
	MainItems      []string `toml:"main_items"`
	IntervalRounds int      `toml:"interval_rounds"`
}

type MQTTConfig struct {
	Root     string `toml:"root"`
	Host     string `toml:"host"`
	Socket   string `toml:"socket"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	ClientID string `toml:"client_id"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}

	// Set defaults
	if cfg.MyGekko.Interval == 0 {
		cfg.MyGekko.Interval = 5.0
	}
	if cfg.MyGekko.IntervalRounds == 0 {
		cfg.MyGekko.IntervalRounds = 4
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	// MyGekko validation
	if c.MyGekko.Host == "" {
		return fmt.Errorf("mygekko.host is required")
	}
	if c.MyGekko.Username == "" {
		return fmt.Errorf("mygekko.username is required")
	}
	if c.MyGekko.Password == "" {
		return fmt.Errorf("mygekko.password is required")
	}
	if c.MyGekko.Interval <= 0 {
		return fmt.Errorf("mygekko.interval must be positive")
	}
	if c.MyGekko.IntervalRounds <= 0 {
		return fmt.Errorf("mygekko.interval_rounds must be positive")
	}
	if len(c.MyGekko.IntervalItems) == 0 && len(c.MyGekko.MainItems) == 0 {
		return fmt.Errorf("at least one of mygekko.interval_items or mygekko.main_items is required")
	}

	// MQTT validation
	if c.MQTT.Host == "" && c.MQTT.Socket == "" {
		return fmt.Errorf("either mqtt.host or mqtt.socket is required")
	}
	if c.MQTT.Root == "" {
		return fmt.Errorf("mqtt.root is required")
	}

	return nil
}
