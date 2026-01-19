package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	content := `
log_level = "DEBUG"

[mygekko]
host = "mygekko.example.com"
username = "user"
password = "pass"
interval = 10.0
interval_items = ["blinds"]
main_items = ["vents"]
interval_rounds = 5

[mqtt]
url = "ssl://mqtt.example.com:8883"
root = "test"
username = "mqttuser"
password = "mqttpass"
`
	path := writeTempConfig(t, content)
	defer os.Remove(path)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != "DEBUG" {
		t.Errorf("expected LogLevel 'DEBUG', got '%s'", cfg.LogLevel)
	}
	if cfg.MyGekko.Host != "mygekko.example.com" {
		t.Errorf("expected MyGekko.Host 'mygekko.example.com', got '%s'", cfg.MyGekko.Host)
	}
	if cfg.MyGekko.Interval != 10.0 {
		t.Errorf("expected MyGekko.Interval 10.0, got %f", cfg.MyGekko.Interval)
	}
	if cfg.MyGekko.IntervalRounds != 5 {
		t.Errorf("expected MyGekko.IntervalRounds 5, got %d", cfg.MyGekko.IntervalRounds)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	content := `
[mygekko]
host = "mygekko.example.com"
username = "user"
password = "pass"
interval_items = ["blinds"]

[mqtt]
url = "tcp://mqtt.example.com:1883"
root = "test"
`
	path := writeTempConfig(t, content)
	defer os.Remove(path)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MyGekko.Interval != 5.0 {
		t.Errorf("expected default Interval 5.0, got %f", cfg.MyGekko.Interval)
	}
	if cfg.MyGekko.IntervalRounds != 4 {
		t.Errorf("expected default IntervalRounds 4, got %d", cfg.MyGekko.IntervalRounds)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	content := `this is not valid toml [[[`
	path := writeTempConfig(t, content)
	defer os.Remove(path)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestValidate_MissingMyGekkoHost(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing mygekko.host")
	}
	if err.Error() != "mygekko.host is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_MissingMyGekkoUsername(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing mygekko.username")
	}
}

func TestValidate_MissingMyGekkoPassword(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing mygekko.password")
	}
}

func TestValidate_InvalidInterval(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       -1.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestValidate_InvalidIntervalRounds(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 0,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for zero interval_rounds")
	}
}

func TestValidate_NoItems(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
		},
		MQTT: MQTTConfig{
			URL:  "tcp://mqtt.example.com:1883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for no items")
	}
}

func TestValidate_MissingMQTTURL(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing mqtt.url")
	}
}

func TestValidate_MQTTUnixSocket(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL:  "unix:///run/mosquitto/mosquitto.sock",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error with unix socket URL: %v", err)
	}
}

func TestValidate_MissingMQTTRoot(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
		},
		MQTT: MQTTConfig{
			URL: "tcp://mqtt.example.com:1883",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing mqtt.root")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		MyGekko: MyGekkoConfig{
			Host:           "mygekko.example.com",
			Username:       "user",
			Password:       "pass",
			Interval:       5.0,
			IntervalRounds: 4,
			IntervalItems:  []string{"blinds"},
			MainItems:      []string{"vents"},
		},
		MQTT: MQTTConfig{
			URL:  "ssl://mqtt.example.com:8883",
			Root: "test",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error for valid config: %v", err)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
