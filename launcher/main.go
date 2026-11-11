// Home Assistant options adapter for the shell-free runtime image.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"syscall"
)

var version = "development"
var revision = "unknown"

type options struct {
	MQTTServer   string      `json:"mqtt_server"`
	MQTTUser     string      `json:"mqtt_user"`
	MQTTPassword string      `json:"mqtt_password"`
	Debug        bool        `json:"debug"`
	LogPings     bool        `json:"log_pings"`
	PHEVIP       string      `json:"phev_ip"`
	PHEVPort     json.Number `json:"phev_port"`
}

func bridgeArgs(data []byte) ([]string, error) {
	var o options
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("invalid options.json")
	}
	if o.MQTTServer == "" || o.PHEVIP == "" || o.PHEVPort == "" {
		return nil, fmt.Errorf("mqtt_server, phev_ip and phev_port must be configured")
	}
	verbosity := "info"
	if o.Debug {
		verbosity = "debug"
	}
	return []string{"/opt/phev2mqtt", "client", "mqtt",
		"--mqtt_server", "tcp://" + o.MQTTServer + "/",
		"--mqtt_username", o.MQTTUser, "--mqtt_password", o.MQTTPassword,
		"--address", net.JoinHostPort(o.PHEVIP, string(o.PHEVPort)),
		// WiFi is managed by the HA host, not this container.
		"--wifi_restart_command", "",
		"--verbosity", verbosity,
		fmt.Sprintf("--log_pings=%t", o.LogPings),
	}, nil
}

func run() error {
	data, err := os.ReadFile("/data/options.json")
	if err != nil {
		return fmt.Errorf("read Home Assistant options: %w", err)
	}
	args, err := bridgeArgs(data)
	if err != nil {
		return err
	}
	fmt.Printf("Starting phev2mqtt: PHEV-DRAGONS %s (source %s)\n", version, revision)
	// Replace PID 1 so the bridge receives Supervisor shutdown signals directly.
	return syscall.Exec(args[0], args, os.Environ())
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
