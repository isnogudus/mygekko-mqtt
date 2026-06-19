package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/BurntSushi/toml"
)

type Config struct {
	LogLevel string        `toml:"log_level"`
	MyGekko  MyGekkoConfig `toml:"mygekko"`
	MQTT     MQTTConfig    `toml:"mqtt"`
	Sandbox  SandboxConfig `toml:"sandbox"`
}

type SandboxConfig struct {
	Chroot string `toml:"chroot"`
	User   string `toml:"user"`
	Group  string `toml:"group"`
}

type MyGekkoConfig struct {
	Host            string   `toml:"host"`
	Username        string   `toml:"username"`
	Password        string   `toml:"password"`
	Interval        float64  `toml:"interval"`
	IntervalItems   []string `toml:"interval_items"`
	MainItems       []string `toml:"main_items"`
	IntervalRounds  int      `toml:"interval_rounds"`
	CommandInterval float64  `toml:"command_interval"`
	// ThrottlePrefixes partitions commands per category into throttled and
	// immediate. For a category listed here, a command is throttled only if its
	// payload starts with one of the given prefixes (e.g. blinds "P50"); every
	// other command (e.g. a STOP) is sent immediately. Categories not listed
	// here are throttled entirely.
	ThrottlePrefixes map[string][]string `toml:"throttle_prefixes"`
}

type MQTTConfig struct {
	Root     string `toml:"root"`
	URL      string `toml:"url"`
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
	if cfg.MyGekko.CommandInterval == 0 {
		cfg.MyGekko.CommandInterval = 20.0
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
	if c.MyGekko.CommandInterval < 0 {
		return fmt.Errorf("mygekko.command_interval must not be negative")
	}
	if len(c.MyGekko.IntervalItems) == 0 && len(c.MyGekko.MainItems) == 0 {
		return fmt.Errorf("at least one of mygekko.interval_items or mygekko.main_items is required")
	}

	// MQTT validation
	if c.MQTT.URL == "" {
		return fmt.Errorf("mqtt.url is required")
	}
	if c.MQTT.Root == "" {
		return fmt.Errorf("mqtt.root is required")
	}

	return nil
}

func lookupUID(name string) (int, error) {
	if name == "" {
		return 0, nil
	}
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("user %s: %w", name, err)
	}
	return strconv.Atoi(u.Uid)
}

func lookupGID(name string) (int, error) {
	if name == "" {
		return 0, nil
	}
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("group %s: %w", name, err)
	}
	return strconv.Atoi(g.Gid)
}
