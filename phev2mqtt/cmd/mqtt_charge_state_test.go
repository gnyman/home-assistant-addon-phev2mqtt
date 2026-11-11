package cmd

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/buxtronix/phev2mqtt/protocol"
)

func feedChargeRegister(m *mqttClient, register byte, data []byte) {
	msg := &protocol.PhevMessage{Type: protocol.CmdInResp, Ack: protocol.Request, Register: register, Data: data}
	if register == protocol.ChargePlugRegister {
		msg.Reg = &protocol.RegisterChargePlug{}
	} else {
		msg.Reg = &protocol.RegisterChargeStatus{}
	}
	msg.Reg.Decode(msg)
	m.publishRegister(msg)
}

func TestChargeStateFromNotifications(t *testing.T) {
	for _, tc := range []struct {
		name         string
		plug, status []byte
		want         string
	}{
		{"charging", []byte{2, 0}, []byte{1, 165, 0}, "charging"},
		{"captured waiting despite charging bit", []byte{2, 1}, []byte{1, 165, 0}, "waiting"},
		{"timer error despite charging bit", []byte{2, 3}, []byte{1, 165, 0}, "error"},
		{"unplugged despite old charging bit", []byte{0, 0}, []byte{1, 165, 0}, "unplugged"},
		{"connected idle", []byte{2, 0}, []byte{0, 0, 0}, "idle"},
		{"waiting without charging data", []byte{2, 1}, nil, "waiting"},
		{"connected without charging data", []byte{2, 0}, nil, "unknown"},
		{"missing plug data", nil, []byte{1, 165, 0}, "unknown"},
		{"malformed plug data", []byte{2}, []byte{1, 165, 0}, "unknown"},
		{"unsupported plug status", []byte{2, 2}, []byte{1, 165, 0}, "unknown"},
		{"unsupported charging status", []byte{2, 0}, []byte{2, 0, 0}, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				broker := &recordingMQTT{}
				m := &mqttClient{client: broker, prefix: "custom/phev", mqttData: map[string]string{}}
				if reverse {
					feedChargeRegister(m, 0x1f, tc.status)
					feedChargeRegister(m, 0x1e, tc.plug)
				} else {
					feedChargeRegister(m, 0x1e, tc.plug)
					feedChargeRegister(m, 0x1f, tc.status)
				}
				if got := m.mqttData["/charge/state"]; got != tc.want {
					t.Fatalf("reverse=%v state=%q want %q", reverse, got, tc.want)
				}
				var available interface{}
				for _, msg := range broker.messages {
					if msg.topic == "custom/phev/charge/state/available" {
						available = msg.payload
					}
				}
				wantAvailable := "online"
				if tc.want == "unknown" {
					wantAvailable = "offline"
				}
				if available != wantAvailable {
					t.Fatalf("availability=%v want %s", available, wantAvailable)
				}
				if got := m.mqttData["/register/1e"]; got != hex.EncodeToString(tc.plug) {
					t.Fatalf("raw plug lost: %q", got)
				}
				if len(tc.status) == 3 && tc.status[0] == 1 && m.mqttData["/charge/charging"] != "on" {
					t.Fatal("legacy charging boolean changed")
				}
			}
		})
	}
}

func TestChargeStateFreshness(t *testing.T) {
	broker := &recordingMQTT{}
	m := &mqttClient{client: broker, prefix: "phev", mqttData: map[string]string{}}
	feedChargeRegister(m, 0x1e, []byte{2, 0})
	feedChargeRegister(m, 0x1f, []byte{1, 165, 0})
	feedChargeRegister(m, 0x1e, []byte{0, 0})
	feedChargeRegister(m, 0x1e, []byte{2, 0})
	if m.mqttData["/charge/state"] != "unknown" {
		t.Fatal("replug reused stale charging data")
	}
	feedChargeRegister(m, 0x1f, []byte{0, 0, 0})
	if m.mqttData["/charge/state"] != "idle" {
		t.Fatal("fresh status did not update")
	}
	for _, reset := range []func(){func() { m.onMqttConnect(broker) }, func() { m.resetChargeState(broker) }} {
		reset()
		if m.chargeState.plug != nil || m.chargeState.status != nil {
			t.Fatal("reconnect preserved previous session")
		}
		last := broker.messages[len(broker.messages)-1]
		if last.topic != "phev/charge/state/available" || last.payload != "offline" || !last.retained {
			t.Fatalf("reset availability: %+v", last)
		}
		feedChargeRegister(m, 0x1f, []byte{1, 165, 0})
		if m.mqttData["/charge/state"] != "unknown" {
			t.Fatal("old plug data survived reconnect")
		}
	}
}

func TestChargeStateDiscovery(t *testing.T) {
	messages := homeAssistantDiscoveryMessages("TESTVIN", "custom/phev", "Phev", "discovery")
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(messages["discovery/sensor/TESTVIN_charge_state/config"]), &config); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]interface{}{"state_topic": "~/charge/state", "unique_id": "TESTVIN_charge_state", "~": "custom/phev", "availability_mode": "all"} {
		if config[key] != want {
			t.Fatalf("%s=%v want %v", key, config[key], want)
		}
	}
	wantAvailability := []interface{}{map[string]interface{}{"topic": "~/available"}, map[string]interface{}{"topic": "~/charge/state/available"}}
	if !reflect.DeepEqual(config["availability"], wantAvailability) {
		t.Fatalf("availability=%v", config["availability"])
	}
	if _, ok := messages["discovery/binary_sensor/TESTVIN_battery_charging/config"]; !ok {
		t.Fatal("legacy charging entity removed")
	}
}
