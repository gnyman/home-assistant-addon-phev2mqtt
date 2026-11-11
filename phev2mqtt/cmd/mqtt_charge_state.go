package cmd

import (
	"bytes"
	"sync"

	"github.com/buxtronix/phev2mqtt/protocol"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type chargeState struct {
	mu           sync.Mutex
	plug, status []byte
}

func (s *chargeState) value() string {
	if len(s.plug) != 2 {
		return "unknown"
	}
	// PHEV Link's RegisterDecoder reads waiting/error from register 0x1e byte 1.
	switch s.plug[1] {
	case 3:
		return "error"
	case 1:
		return "waiting"
	case 0:
	default:
		return "unknown"
	}
	if s.plug[0] == 0 {
		return "unplugged"
	}
	if len(s.status) != 3 {
		return "unknown"
	}
	switch s.status[0] {
	case 1:
		return "charging"
	case 0:
		return "idle"
	default:
		return "unknown"
	}
}

func (m *mqttClient) resetChargeState(c mqtt.Client) {
	m.chargeState.mu.Lock()
	defer m.chargeState.mu.Unlock()
	m.chargeState.plug, m.chargeState.status = nil, nil
	c.Publish(m.topic("/charge/state"), 0, false, "unknown")
	c.Publish(m.topic("/charge/state/available"), 0, true, "offline")
}

func (m *mqttClient) publishChargeState(msg *protocol.PhevMessage) {
	if msg.Register != protocol.ChargePlugRegister && msg.Register != protocol.ChargeStatusRegister {
		return
	}
	m.chargeState.mu.Lock()
	defer m.chargeState.mu.Unlock()
	data := append([]byte(nil), msg.Data...)
	if msg.Register == protocol.ChargePlugRegister {
		if len(m.chargeState.plug) == 2 && !bytes.Equal(m.chargeState.plug, data) {
			m.chargeState.status = nil
		}
		m.chargeState.plug = data
	} else {
		m.chargeState.status = data
	}
	state := m.chargeState.value()
	m.publish("/charge/state", state)
	available := "offline"
	if state != "unknown" {
		available = "online"
		m.client.Publish(m.topic("/available"), 0, true, "online")
	}
	m.client.Publish(m.topic("/charge/state/available"), 0, true, available)
}
