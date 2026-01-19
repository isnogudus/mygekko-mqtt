package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logging
	SetupLogger(cfg.LogLevel)
	slog.Info("Starting mygekko-mqtt bridge")

	// Create clients
	gekko := NewMyGekkoClient(cfg.MyGekko)

	// Load field definitions from MyGEKKO
	fieldDefinitions, err := LoadFieldDefinitions(gekko)
	if err != nil {
		slog.Error("Failed to parse definitions", "error", err)
		os.Exit(2)
	}

	mqtt, err := NewMQTTClient(cfg.MQTT)
	if err != nil {
		slog.Error("Failed to connect to MQTT", "error", err)
		os.Exit(1)
	}
	defer mqtt.Disconnect()

	// Create and start bridge
	bridge, err := NewBridge(cfg, gekko, mqtt, fieldDefinitions)
	if err != nil {
		slog.Error("Failed to create bridge", "error", err)
		os.Exit(1)
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go bridge.RunGetter()
	go bridge.RunSetter()

	// Wait for shutdown signal
	sig := <-sigChan
	slog.Info("Received signal, shutting down", "signal", sig)
	bridge.Stop()
}
