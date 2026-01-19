# mygekko-mqtt

A bridge that connects MyGEKKO home automation systems to MQTT, enabling integration with Home Assistant, Node-RED, and other MQTT-based systems.

## Features

- Polls MyGEKKO API and publishes status values to MQTT
- Subscribes to MQTT commands and forwards them to MyGEKKO
- Automatic field definition loading from MyGEKKO API
- History-based deduplication (only publishes changed values)
- JSON publishing with timestamps for each item
- Supports both TCP (TLS) and Unix socket MQTT connections
- Configurable polling intervals
- Structured logging with configurable log levels

## Installation

```bash
go build -o mygekko-mqtt .
```

### Production Build

For a smaller, optimized binary:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o mygekko-mqtt .
```

Cross-compile for other platforms:

```bash
# OpenBSD
CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -ldflags="-s -w" -o mygekko-mqtt-openbsd .

# Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o mygekko-mqtt-linux .
```

## Configuration

Copy the sample configuration and edit it:

```bash
cp config.toml.sample config.toml
```

### Configuration Options

```toml
# Log level: DEBUG, INFO, WARN, ERROR (default: INFO)
log_level = "INFO"

[mygekko]
# MyGEKKO device hostname or IP
host = "192.168.1.100"

# MyGEKKO credentials
username = "admin"
password = "secret"

# Polling interval in seconds (default: 5.0)
interval = 5.0

# Number of interval rounds before polling main_items (default: 4)
interval_rounds = 4

# Categories to poll every interval (e.g., fast-changing values)
interval_items = ["blinds", "lights"]

# Categories to poll less frequently (every interval_rounds)
main_items = ["vents", "energycosts"]

[mqtt]
# MQTT broker URL
# Supported schemes:
#   tcp://host:port      - Plain TCP (default port 1883)
#   ssl://host:port      - TLS/SSL (default port 8883)
#   unix:///path/to/sock - Unix socket
url = "ssl://mqtt.example.com:8883"

# MQTT topic root prefix
root = "mygekko"

# MQTT credentials (optional for some brokers)
username = "mqttuser"
password = "mqttpass"

# Client ID (optional, default: "mygekko-mqtt")
client_id = "mygekko-mqtt"
```

## Usage

```bash
# Using default config.toml in current directory
./mygekko-mqtt

# Specify config file path
./mygekko-mqtt -config /etc/mygekko-mqtt/config.toml
```

## MQTT Topics

### Published Topics (Status)

```
{root}/{gekkoname}/{category}/{item}/get/{field}    # Individual field values
{root}/{gekkoname}/{category}/{item}/get/json       # JSON with all fields + timestamp
{root}/{gekkoname}/{category}/get/time              # Polling timestamp per category
{root}/{gekkoname}/getter_online                    # Bridge getter status
{root}/{gekkoname}/setter_online                    # Bridge setter status
```

Example:
```
mygekko/MyHome/blinds/item0/get/position     -> 50
mygekko/MyHome/blinds/item0/get/angle        -> 45.5
mygekko/MyHome/blinds/item0/get/json         -> {"position":50,"angle":45.5,"timestamp":1705123456}
```

### Subscribed Topics (Commands)

```
{root}/{gekkoname}/{category}/{item}/set
```

Example:
```
mygekko/MyHome/blinds/item0/set    <- "P50"   # Set position to 50%
```

## Exit Codes

The application uses distinct exit codes for different failure scenarios, making it suitable for process supervisors like runit or systemd:

| Code | Description |
|------|-------------|
| 1 | Configuration, MQTT connection, or bridge initialization error |
| 2 | Failed to load field definitions from MyGEKKO |
| 5 | Value parsing error |
| 6 | MQTT publish error |
| 7 | MQTT subscribe error |
| 8 | Invalid MQTT topic format |
| 9 | MyGEKKO SetValue command error |
| 10 | MQTT connection lost |
| 11 | MyGEKKO connection lost during polling |

## Running with runit

Create a run script at `/etc/sv/mygekko-mqtt/run`:

```bash
#!/bin/sh
exec 2>&1
exec chpst -u mygekko /usr/local/bin/mygekko-mqtt -config /etc/mygekko-mqtt/config.toml
```

Enable the service:

```bash
ln -s /etc/sv/mygekko-mqtt /var/service/
```

## Running with systemd

Create `/etc/systemd/system/mygekko-mqtt.service`:

```ini
[Unit]
Description=MyGEKKO MQTT Bridge
After=network.target

[Service]
Type=simple
User=mygekko
ExecStart=/usr/local/bin/mygekko-mqtt -config /etc/mygekko-mqtt/config.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
systemctl enable mygekko-mqtt
systemctl start mygekko-mqtt
```

## Development

### Running Tests

```bash
go test -v ./...
```

### Running with Development Config

```bash
# Use a separate client_id to run alongside production
./mygekko-mqtt -config config-dev.toml
```

## License

MIT
