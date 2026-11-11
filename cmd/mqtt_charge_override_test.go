package cmd

import (
	"encoding/json"
	"errors"
	"github.com/buxtronix/phev2mqtt/protocol"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"reflect"
	"strings"
	"testing"
)

type fakeRegisterSetter struct {
	calls []fakeRegisterCall
	err   error
}

type fakeRegisterCall struct {
	register byte
	value    []byte
}

func (f *fakeRegisterSetter) SetRegister(register byte, value []byte) error {
	f.calls = append(f.calls, fakeRegisterCall{register: register, value: append([]byte(nil), value...)})
	return f.err
}

func TestChargeTimerOverrideValidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		payload string
		want    []byte
	}{
		{name: "new topic on", topic: "phev/set/charge/override", payload: "on", want: []byte{0x01}},
		{name: "new topic on uppercase", topic: "phev/set/charge/override", payload: "ON", want: []byte{0x01}},
		{name: "new topic on whitespace", topic: "phev/set/charge/override", payload: "  on\n", want: []byte{0x01}},
		{name: "new topic off", topic: "phev/set/charge/override", payload: "off", want: []byte{0x02}},
		{name: "new topic off mixed case", topic: "phev/set/charge/override", payload: "Off", want: []byte{0x02}},
		{name: "legacy topic on", topic: "phev/set/cancelchargetimer", payload: "on", want: []byte{0x01}},
		{name: "legacy topic off", topic: "phev/set/cancelchargetimer", payload: "off", want: []byte{0x02}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setter := &fakeRegisterSetter{}
			handled, err := handleChargeTimerOverride(tc.topic, "phev", []byte(tc.payload), setter)
			if !handled {
				t.Fatalf("handleChargeTimerOverride() did not handle topic %q", tc.topic)
			}
			if err != nil {
				t.Fatalf("handleChargeTimerOverride() unexpected error: %v", err)
			}
			if len(setter.calls) != 1 {
				t.Fatalf("SetRegister called %d times, want exactly 1: %+v", len(setter.calls), setter.calls)
			}
			got := setter.calls[0]
			if got.register != 0x17 {
				t.Errorf("SetRegister register = 0x%02x, want 0x17", got.register)
			}
			if !reflect.DeepEqual(got.value, tc.want) {
				t.Errorf("SetRegister data = %v, want %v", got.value, tc.want)
			}
		})
	}
}

func TestChargeTimerOverrideInvalidPayloadsWriteNothing(t *testing.T) {
	payloads := []string{
		"",
		" ",
		"\t\n",
		"1",
		"2",
		"01",
		"02",
		"0x01",
		"0x02",
		"0",
		"3",
		"true",
		"false",
		"enable",
		"disable",
		"cancel",
		"onn",
		"offf",
	}
	topics := []string{"phev/set/charge/override", "phev/set/cancelchargetimer"}
	for _, topic := range topics {
		for _, payload := range payloads {
			t.Run(topic+"_"+payload, func(t *testing.T) {
				setter := &fakeRegisterSetter{}
				handled, err := handleChargeTimerOverride(topic, "phev", []byte(payload), setter)
				if !handled {
					t.Fatalf("handleChargeTimerOverride() did not handle topic %q", topic)
				}
				if !errors.Is(err, errInvalidChargeTimerOverride) {
					t.Fatalf("handleChargeTimerOverride(%q) error = %v, want errInvalidChargeTimerOverride", payload, err)
				}
				if len(setter.calls) != 0 {
					t.Fatalf("SetRegister called %d times for invalid payload %q, want 0: %+v", len(setter.calls), payload, setter.calls)
				}
			})
		}
	}
}

func TestChargeTimerOverrideIgnoresOtherTopics(t *testing.T) {
	setter := &fakeRegisterSetter{}
	handled, err := handleChargeTimerOverride("phev/set/charge/other", "phev", []byte("on"), setter)
	if handled {
		t.Fatal("handleChargeTimerOverride() handled an unrelated topic")
	}
	if err != nil {
		t.Fatalf("handleChargeTimerOverride() unexpected error: %v", err)
	}
	if len(setter.calls) != 0 {
		t.Fatalf("SetRegister called %d times, want 0", len(setter.calls))
	}
}

func TestChargeTimerOverrideDiscovery(t *testing.T) {
	const (
		vin           = "JMFXDGG2WJZ00048"
		wantTopic     = "homeassistant/switch/" + vin + "_cancel_charge_timer/config"
		wantUniqueID  = vin + "_cancel_charge_timer"
		oldStateTopic = "~/battery/charging"
	)

	messages := homeAssistantDiscoveryMessages(vin, "phev", "Phev", "homeassistant")
	raw, ok := messages[wantTopic]
	if !ok {
		keys := make([]string, 0, len(messages))
		for topic := range messages {
			keys = append(keys, topic)
		}
		t.Fatalf("discovery topic %q not found; got %v", wantTopic, keys)
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatalf("switch discovery payload is not valid JSON: %v\n%s", err, raw)
	}

	want := map[string]interface{}{
		"name":              "Phev Charge Timer Override (experimental)",
		"icon":              "mdi:timer-sync-outline",
		"state_topic":       "~/charge/override",
		"command_topic":     "~/set/charge/override",
		"payload_on":        "on",
		"payload_off":       "off",
		"availability_mode": "all",
		"unique_id":         wantUniqueID,
		"~":                 "phev",
	}
	for key, wantValue := range want {
		if got := config[key]; got != wantValue {
			t.Errorf("switch discovery %q = %v, want %v", key, got, wantValue)
		}
	}
	wantAvailability := []interface{}{map[string]interface{}{"topic": "~/available"}, map[string]interface{}{"topic": "~/charge/override/available"}}
	if !reflect.DeepEqual(config["availability"], wantAvailability) {
		t.Fatalf("incorrect availability: %v", config["availability"])
	}
	if _, exists := config["avty_t"]; exists {
		t.Fatal("single availability topic overrides freshness checks")
	}
	if strings.Contains(raw, oldStateTopic) {
		t.Errorf("switch discovery still contains old charging state topic %q", oldStateTopic)
	}

	for topic, payload := range messages {
		if strings.Contains(payload, oldStateTopic) {
			t.Errorf("discovery payload for %q contains old charging state topic %q", topic, oldStateTopic)
		}
		var candidate map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &candidate); err != nil {
			continue
		}
		if candidate["unique_id"] == wantUniqueID && topic != wantTopic {
			t.Errorf("duplicate switch discovery for unique_id %q at %q", wantUniqueID, topic)
		}
	}
}

type recordedPublish struct {
	topic    string
	payload  interface{}
	retained bool
}
type recordingMQTT struct {
	mqtt.Client
	messages []recordedPublish
}

func (c *recordingMQTT) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.messages = append(c.messages, recordedPublish{topic, payload, retained})
	return nil
}

func TestOverrideNotificationsAndAvailability(t *testing.T) {
	broker := &recordingMQTT{}
	m := &mqttClient{client: broker, prefix: "custom/phev", mqttData: map[string]string{}}
	for _, tc := range []struct {
		data                  []byte
		state, available, raw string
	}{
		{[]byte{0x12}, "off", "online", "12"},
		{[]byte{0}, "unknown", "offline", "00"},
		{nil, "unknown", "offline", ""},
		{[]byte{0x11}, "on", "online", "11"},
	} {
		broker.messages = nil
		m.publishRegister(&protocol.PhevMessage{Type: protocol.CmdInResp, Ack: protocol.Request, Register: 0x1b, Data: tc.data})
		msgs := broker.messages
		if len(msgs) < 3 || msgs[0].topic != "custom/phev/register/1b" || msgs[0].payload != tc.raw || msgs[1].topic != "custom/phev/charge/override" || msgs[1].payload != tc.state {
			t.Fatalf("wrong register/state publications: %+v", msgs)
		}
		last := msgs[len(msgs)-1]
		if last.topic != "custom/phev/charge/override/available" || last.payload != tc.available || !last.retained {
			t.Fatalf("wrong availability: %+v", msgs)
		}
	}
	broker.messages = nil
	m.onMqttConnect(broker)
	if len(broker.messages) < 1 || broker.messages[0].payload != "offline" || !broker.messages[0].retained {
		t.Fatalf("reconnect did not invalidate old status: %+v", broker.messages)
	}
}

func TestOverrideTimeoutPropagates(t *testing.T) {
	timeout := errors.New("timed out attempting to set register 17")
	setter := &fakeRegisterSetter{err: timeout}
	handled, err := handleChargeTimerOverride("custom/phev/set/charge/override", "custom/phev", []byte("OFF"), setter)
	if !handled || !errors.Is(err, timeout) || len(setter.calls) != 1 {
		t.Fatalf("lost error or retried write: handled=%v err=%v calls=%v", handled, err, setter.calls)
	}
}
