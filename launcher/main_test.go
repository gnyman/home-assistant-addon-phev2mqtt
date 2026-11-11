package main

import (
	"reflect"
	"testing"
)

func TestBridgeArgs(t *testing.T) {
	args, debug, err := bridgeArgs([]byte(`{"mqtt_server":"broker:1883","mqtt_user":"a b","mqtt_password":"$secret; x","phev_ip":"192.168.8.46","phev_port":"8080"}`))
	want := []string{"/opt/phev2mqtt", "client", "mqtt", "--mqtt_server", "tcp://broker:1883/", "--mqtt_username", "a b", "--mqtt_password", "$secret; x", "--address", "192.168.8.46:8080", "--wifi_restart_command", ""}
	if err != nil || debug || !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected argument mapping or error: %v", err)
	}
}
func TestNullCredentials(t *testing.T) {
	args, _, err := bridgeArgs([]byte(`{"mqtt_server":"broker:1883","mqtt_user":null,"mqtt_password":null,"phev_ip":"::1","phev_port":8080}`))
	if err != nil || args[6] != "" || args[8] != "" || args[10] != "[::1]:8080" {
		t.Fatal("null credentials or IPv6 not handled")
	}
}
func TestDebugAndInvalidOptions(t *testing.T) {
	_, debug, err := bridgeArgs([]byte(`{"debug":true}`))
	if err != nil || !debug {
		t.Fatal("debug should not require connection options")
	}
	for _, input := range []string{`{`, `{}`, `{"mqtt_server":123}`, `{"debug":"false"}`} {
		if _, _, err := bridgeArgs([]byte(input)); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
