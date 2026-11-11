// Home Assistant options adapter for the shell-free runtime image.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type options struct {
	MQTTServer   string      `json:"mqtt_server"`
	MQTTUser     string      `json:"mqtt_user"`
	MQTTPassword string      `json:"mqtt_password"`
	Debug        bool        `json:"debug"`
	PHEVIP       string      `json:"phev_ip"`
	PHEVPort     json.Number `json:"phev_port"`
}

func bridgeArgs(data []byte) ([]string, bool, error) {
	var o options
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, false, fmt.Errorf("invalid options.json")
	}
	if o.Debug {
		return nil, true, nil
	}
	if o.MQTTServer == "" || o.PHEVIP == "" || o.PHEVPort == "" {
		return nil, false, fmt.Errorf("mqtt_server, phev_ip and phev_port must be configured")
	}
	return []string{"/opt/phev2mqtt", "client", "mqtt",
		"--mqtt_server", "tcp://" + o.MQTTServer + "/",
		"--mqtt_username", o.MQTTUser, "--mqtt_password", o.MQTTPassword,
		"--address", net.JoinHostPort(o.PHEVIP, string(o.PHEVPort)),
		// WiFi is managed by the HA host, not this container.
		"--wifi_restart_command", "",
	}, false, nil
}

func run() error {
	data, err := os.ReadFile("/data/options.json")
	if err != nil {
		return fmt.Errorf("read Home Assistant options: %w", err)
	}
	args, debug, err := bridgeArgs(data)
	if err != nil {
		return err
	}
	if debug {
		fmt.Println("Debug mode on - bridge stopped; use /opt/phev2mqtt directly for diagnostics")
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
		<-stop
		signal.Stop(stop)
		return nil
	}
	fmt.Println("Starting phev2mqtt")
	// Replace PID 1 so the bridge receives Supervisor shutdown signals directly.
	return syscall.Exec(args[0], args, os.Environ())
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
