//go:build openbsd

package main

import (
	"log/slog"

	"golang.org/x/sys/unix"
)

func pledge() error {
	// stdio: standard I/O
	// rpath: read files (/dev/urandom for crypto, /etc/localtime for timezone)
	// inet: network sockets (MyGEKKO and MQTT)
	// dns: DNS resolution
	// unix: unix sockets (MQTT)
	err := unix.Pledge("stdio rpath inet dns unix", "")
	if err == nil {
		slog.Debug("pledge", "promises", "stdio rpath inet dns unix")
	}
	return err
}
