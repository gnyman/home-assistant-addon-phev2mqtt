package cmd

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/buxtronix/phev2mqtt/protocol"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestHandshakeFailureClosesSocket(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, e := l.Accept()
		if e == nil {
			accepted <- conn
		}
	}()
	viper.Set("address", l.Addr().String())
	defer viper.Set("address", nil)
	broker := &recordingMQTT{}
	m := &mqttClient{client: broker, prefix: "test", mqttData: map[string]string{}}
	// Intentionally withhold the handshake; use the real startup timeout.
	err = m.handlePhev(&cobra.Command{})
	if m.phev != nil {
		defer m.phev.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for start") {
		t.Fatalf("unexpected result: %v", err)
	}
	peer := <-accepted
	defer peer.Close()
	peer.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		var data [1024]byte
		_, err = peer.Read(data[:])
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("failed handshake left socket alive: %v", err)
		}
	}
}

func TestDialFailureClosesClient(t *testing.T) {
	viper.Set("address", "missing-port")
	defer viper.Set("address", nil)
	m := &mqttClient{client: &recordingMQTT{}, prefix: "test", mqttData: map[string]string{}}
	if err := m.handlePhev(&cobra.Command{}); err == nil {
		t.Fatal("invalid endpoint succeeded")
	}
	if m.phev == nil {
		t.Fatal("client was not created")
	}
	defer m.phev.Close()
	listener := m.phev.AddListener()
	select {
	case _, ok := <-listener.C:
		if ok {
			t.Fatal("failed dial left client open")
		}
	case <-time.After(time.Second):
		t.Fatal("failed dial did not close client")
	}
}

func TestPhevRetryDelayCapsAtThreeMinutes(t *testing.T) {
	var delay time.Duration
	for _, seconds := range []time.Duration{1, 2, 4, 8, 16, 32, 64, 128, 180, 180} {
		delay = nextPhevRetryDelay(delay)
		if want := seconds * time.Second; delay != want {
			t.Fatalf("retry delay = %s, want %s", delay, want)
		}
	}
}

type reconnectTestMessage struct {
	mqtt.Message
	topic   string
	payload []byte
}

func (m reconnectTestMessage) Topic() string   { return m.topic }
func (m reconnectTestMessage) Payload() []byte { return m.payload }

func TestPhevAcceptsCommandsAfterFailedDial(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	viper.Set("address", "missing-port")
	defer viper.Set("address", nil)

	broker := &recordingMQTT{}
	m := &mqttClient{client: broker, prefix: "test", updateInterval: time.Minute}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := &cobra.Command{RunE: func(cmd *cobra.Command, _ []string) error { return m.handlePhev(cmd) }}
	defer cancel()

	command := reconnectTestMessage{topic: "test/set/register/05", payload: []byte("12345678")}
	if err := m.handlePhev(cmd); err == nil {
		t.Fatal("invalid endpoint succeeded")
	}
	m.handleIncomingMqtt(broker, command)
	m.retryDelay = 3 * time.Minute

	viper.Set("address", listener.Addr().String())
	result := make(chan error, 1)
	go func() { result <- cmd.ExecuteContext(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Error(err)
			}
			if m.retryDelay != 0 {
				t.Errorf("retry delay after successful handshake = %s, want 0", m.retryDelay)
			}
		case <-time.After(3 * time.Second):
			t.Error("PHEV session did not exit")
		}
	}()
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(3 * time.Second))
	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() })
	peer.SetDeadline(time.Now().Add(3 * time.Second))
	key := &protocol.SecurityKey{}
	start := protocol.NewMessage(protocol.CmdInMy14StartReq, 1, false, []byte{0})
	if _, err := peer.Write(start.EncodeToBytes(key)); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		m.phevMu.Lock()
		ready := m.phevReady
		m.phevMu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("PHEV handshake never became ready")
		}
		time.Sleep(time.Millisecond)
	}

	commandDone := make(chan struct{})
	go func() {
		defer close(commandDone)
		m.handleIncomingMqtt(broker, command)
	}()
	for {
		var header [2]byte
		if _, err := io.ReadFull(peer, header[:]); err != nil {
			t.Fatal(err)
		}
		body := make([]byte, header[1])
		if _, err := io.ReadFull(peer, body); err != nil {
			t.Fatal(err)
		}
		if header[0] != protocol.CmdOutSend || len(body) < 3 || body[1] != 5 {
			continue
		}
		if got := string(body[2 : len(body)-1]); got != "\x12\x34\x56\x78" {
			t.Fatalf("register write data = %x", got)
		}
		ack := protocol.NewMessage(protocol.CmdInResp, 5, true, []byte{0})
		if _, err := peer.Write(ack.EncodeToBytes(key)); err != nil {
			t.Fatal(err)
		}
		break
	}
	select {
	case <-commandDone:
	case <-time.After(3 * time.Second):
		t.Fatal("PHEV command did not finish after ACK")
	}
	cancel()
}
