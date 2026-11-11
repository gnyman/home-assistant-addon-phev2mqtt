package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net"
	"strings"
	"testing"
	"time"
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
