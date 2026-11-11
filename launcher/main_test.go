package main

import (
	"reflect"
	"testing"
)

func TestBridgeArgs(t *testing.T) {
	args, err := bridgeArgs([]byte(`{"mqtt_server":"broker:1883","mqtt_user":"a b","mqtt_password":"$secret; x","phev_ip":"192.168.8.46","phev_port":"8080"}`))
	want := []string{"/opt/phev2mqtt", "client", "mqtt", "--mqtt_server", "tcp://broker:1883/", "--mqtt_username", "a b", "--mqtt_password", "$secret; x", "--address", "192.168.8.46:8080", "--wifi_restart_command", "", "--verbosity", "info", "--log_pings=false"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected argument mapping or error: %v", err)
	}
}
func TestNullCredentials(t *testing.T) {
	args, err := bridgeArgs([]byte(`{"mqtt_server":"broker:1883","mqtt_user":null,"mqtt_password":null,"phev_ip":"::1","phev_port":8080}`))
	if err != nil || args[6] != "" || args[8] != "" || args[10] != "[::1]:8080" {
		t.Fatal("null credentials or IPv6 not handled")
	}
}
func TestDebugStartsBridgeWithQuietPings(t *testing.T) {
	args, err := bridgeArgs([]byte(`{"debug":true,"mqtt_server":"broker:1883","phev_ip":"car","phev_port":8080}`))
	if err != nil || args[14] != "debug" || args[15] != "--log_pings=false" {
		t.Fatal("debug must start bridge with protocol logging, keeping pings quiet")
	}
	args, err = bridgeArgs([]byte(`{"debug":true,"log_pings":true,"mqtt_server":"broker:1883","phev_ip":"car","phev_port":8080}`))
	if err != nil || args[15] != "--log_pings=true" {
		t.Fatal("ping logging opt-in ignored")
	}
}
func TestInvalidOptions(t *testing.T) {
	for _, input := range []string{`{`, `{}`, `{"debug":true}`, `{"mqtt_server":123}`, `{"debug":"false"}`, `{"log_pings":"true"}`} {
		if _, err := bridgeArgs([]byte(input)); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
