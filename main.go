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

	// Lookup user/group before chroot (needs /etc/passwd, /etc/group)
	uid, err := lookupUID(cfg.Sandbox.User)
	if err != nil {
		slog.Error("Failed to lookup user", "error", err)
		os.Exit(2)
	}
	gid, err := lookupGID(cfg.Sandbox.Group)
	if err != nil {
		slog.Error("Failed to lookup group", "error", err)
		os.Exit(2)
	}

	// Create MyGEKKO client and load data before sandbox (needs DNS resolution)
	gekko := NewMyGekkoClient(cfg.MyGekko)

	// Get gekko name first (needed for MQTT LWT topic)
	gekkoName, err := gekko.GetGekkoName()
	if err != nil {
		slog.Error("Failed to get gekko name", "error", err)
		os.Exit(4)
	}
	slog.Info("Gekko name", "name", gekkoName)

	// Load field definitions from MyGEKKO
	fieldDefinitions, err := LoadFieldDefinitions(gekko)
	if err != nil {
		slog.Error("Failed to parse definitions", "error", err)
		os.Exit(4)
	}

	// Connect to MQTT with LWT (Last Will Testament)
	mqtt, err := NewMQTTClient(cfg.MQTT, gekkoName)
	if err != nil {
		slog.Error("Failed to connect to MQTT", "error", err)
		os.Exit(5)
	}
	defer mqtt.Disconnect()

	// Sandbox: chroot, drop privileges, pledge
	// Done after connections are established - only needs existing sockets
	if err := sandbox(cfg.Sandbox.Chroot, uid, gid); err != nil {
		slog.Error("Failed to sandbox", "error", err)
		os.Exit(3)
	}

	// Create and start bridge
	bridge, err := NewBridge(cfg, gekko, mqtt, fieldDefinitions, gekkoName)
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
