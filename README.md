# mygekko-mqtt

A bridge that connects MyGEKKO home automation systems to MQTT, enabling integration with Home Assistant, Node-RED, and other MQTT-based systems.

## Features

- Polls MyGEKKO API and publishes status values to MQTT
- Subscribes to MQTT commands and forwards them to MyGEKKO
- Automatic field definition loading from MyGEKKO API
- History-based deduplication (only publishes changed values)
- JSON publishing with timestamps for each item
- Supports both TCP (TLS) and Unix socket MQTT connections
- MQTT Last Will and Testament (LWT) for online/offline status
- Configurable polling intervals
- Structured logging with configurable log levels
- Security sandboxing (chroot, privilege dropping, OpenBSD pledge)

## Requirements

- Go 1.24.0
- MyGEKKO device with API access
- MQTT broker (Mosquitto, etc.)

## Installation

```bash
make build
```

### Cross-compilation

```bash
make build-all      # All platforms
make build-linux    # Linux amd64 + arm64
make build-openbsd  # OpenBSD amd64
make build-darwin   # macOS amd64 + arm64
```

Or manually:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o mygekko-mqtt
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

### Security Sandboxing

The application supports chroot, privilege dropping, and OpenBSD pledge for defense in depth:

```toml
[sandbox]
chroot = "/var/empty"   # chroot directory (requires root)
user = "_mygekko"       # drop to this user after chroot
group = "_mygekko"      # drop to this group after chroot
```

On OpenBSD, the application restricts itself to `stdio rpath inet dns unix` pledges.

The sandbox is applied after establishing all network connections (MyGEKKO API, MQTT), so DNS resolution works normally. After sandboxing, only the existing sockets are used.

## Running

```bash
# Using default config.toml in current directory
./mygekko-mqtt

# Specify config file path
./mygekko-mqtt -config /etc/mygekko-mqtt/config.toml
```

The application follows a "let it crash" philosophy - on errors, it exits with a specific code and should be restarted by a supervisor (systemd, runit, Docker, etc.).

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown |
| 1 | Configuration or bridge initialization error |
| 2 | User/group lookup error |
| 3 | Sandbox error (chroot/setuid/pledge) |
| 4 | MyGEKKO connection error (name or definitions) |
| 5 | MQTT connection error |
| 6 | MQTT publish error |
| 7 | MQTT subscribe error |
| 8 | Invalid MQTT topic format |
| 9 | MyGEKKO SetValue command error |
| 10 | MQTT connection lost |
| 11 | MyGEKKO connection lost during polling |

### Systemd Service

```ini
[Unit]
Description=MyGEKKO MQTT Bridge
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/mygekko-mqtt -config /etc/mygekko-mqtt/config.toml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Runit Service

Create `/etc/sv/mygekko-mqtt/run`:

```bash
#!/bin/sh
exec /usr/local/bin/mygekko-mqtt -config /etc/mygekko-mqtt/config.toml 2>&1
```

## Installation on OpenBSD

### Create service user

```bash
useradd -s /sbin/nologin -d /var/empty -g =uid -c "MyGEKKO MQTT" _mygekko
```

### Copy binary and config

```bash
cp mygekko-mqtt-openbsd-amd64 /usr/local/bin/mygekko-mqtt
chmod 755 /usr/local/bin/mygekko-mqtt

mkdir -p /etc/mygekko-mqtt
cp config.toml /etc/mygekko-mqtt/config.toml
chmod 640 /etc/mygekko-mqtt/config.toml
chown root:_mygekko /etc/mygekko-mqtt/config.toml
```

Add sandbox settings to `/etc/mygekko-mqtt/config.toml`:

```toml
[sandbox]
chroot = "/var/empty"
user = "_mygekko"
group = "_mygekko"
```

### rc.d

Create `/etc/rc.d/mygekkomqtt`:

```bash
#!/bin/ksh

daemon="/usr/local/bin/mygekko-mqtt"
daemon_flags="-config /etc/mygekko-mqtt/config.toml"

. /etc/rc.d/rc.subr

rc_bg=YES

rc_cmd $1
```

```bash
chmod 755 /etc/rc.d/mygekkomqtt
rcctl enable mygekkomqtt
rcctl start mygekkomqtt
```

### Runit (alternative)

```bash
mkdir -p /etc/sv/mygekko-mqtt

cat > /etc/sv/mygekko-mqtt/run << 'EOF'
#!/bin/sh
exec /usr/local/bin/mygekko-mqtt -config /etc/mygekko-mqtt/config.toml 2>&1
EOF

chmod 755 /etc/sv/mygekko-mqtt/run
ln -s /etc/sv/mygekko-mqtt /var/service/
```

## MQTT Topics

### Published Topics (Status)

```
{root}/{gekkoname}/online                           # "true"/"false" (retained, LWT)
{root}/{gekkoname}/{category}/{item}/get/{field}    # Individual field values
{root}/{gekkoname}/{category}/{item}/get/json       # JSON with all fields + timestamp
{root}/{gekkoname}/{category}/get/time              # Polling timestamp per category
```

The `online` topic uses MQTT Last Will and Testament (LWT): it is set to "true" (retained) on connect and the broker automatically publishes "false" if the client disconnects unexpectedly.

Example:
```
mygekko/MyHome/online                        -> true (retained)
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

## Development

### Running Tests

```bash
make test
```

## License

MIT
