package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	client mqtt.Client
	root   string
}

func NewMQTTClient(cfg MQTTConfig) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions()

	// Parse the URL to determine connection type
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT URL: %w", err)
	}

	slog.Info("Connecting to MQTT", "url", cfg.URL)

	// Handle Unix socket connections
	if parsedURL.Scheme == "unix" {
		socketPath := parsedURL.Path
		slog.Info("Using Unix socket", "path", socketPath)
		opts.SetCustomOpenConnectionFn(func(uri *url.URL, options mqtt.ClientOptions) (net.Conn, error) {
			slog.Debug("Opening Unix socket connection", "path", socketPath)
			return net.Dial("unix", socketPath)
		})
		// paho needs a broker URL, use tcp://localhost as dummy since we override the connection
		opts.AddBroker("tcp://localhost:1883")
	} else {
		opts.AddBroker(cfg.URL)
	}
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = "mygekko-mqtt"
	}
	opts.SetClientID(clientID)
	slog.Info("MQTT client ID", "client_id", clientID)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		if err != nil {
			slog.Error("Unexpected MQTT disconnection. Will exit", "error", err)
			os.Exit(10)
		} else {
			slog.Info("Expected MQTT disconnection. Will auto-reconnect")
		}
	})

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("Connected to MQTT")
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("MQTT connection failed: %w", token.Error())
	}

	return &MQTTClient{
		client: client,
		root:   cfg.Root,
	}, nil
}

func (m *MQTTClient) Publish(topic string, value any) error {
	fullTopic := fmt.Sprintf("%s/%s", m.root, topic)
	token := m.client.Publish(fullTopic, 0, false, fmt.Sprintf("%v", value))
	token.Wait()
	return token.Error()
}

func (m *MQTTClient) PublishJSON(topic string, data any) error {
	fullTopic := fmt.Sprintf("%s/%s", m.root, topic)
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	token := m.client.Publish(fullTopic, 0, false, jsonBytes)
	token.Wait()
	return token.Error()
}

func (m *MQTTClient) Subscribe(topic string, handler func(topic string, payload []byte)) error {
	fullTopic := fmt.Sprintf("%s/%s", m.root, topic)
	token := m.client.Subscribe(fullTopic, 0, func(c mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
	})
	token.Wait()
	return token.Error()
}

func (m *MQTTClient) Disconnect() {
	m.client.Disconnect(1000)
}
