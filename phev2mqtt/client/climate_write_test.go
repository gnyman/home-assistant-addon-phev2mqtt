package client

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/buxtronix/phev2mqtt/protocol"
)

// Only in-memory sockets: these tests must never connect to a vehicle.
func TestClimateWriteRequiresMatchingACK(t *testing.T) {
	c, _ := New()
	conn, peer := net.Pipe()
	c.conn = conn
	defer c.Close()
	defer peer.Close()
	go c.reader()
	go c.writer()
	go func() {
		for range c.Recv {
		}
	}()
	result := make(chan error, 1)
	go func() { result <- c.SetRegister(0x1b, []byte{2, 3, 2, 0}) }()
	peer.SetDeadline(time.Now().Add(2 * time.Second))
	raw := make([]byte, 9)
	if _, err := io.ReadFull(peer, raw); err != nil {
		t.Fatal(err)
	}
	msg := &protocol.PhevMessage{}
	if err := msg.DecodeFromBytes(raw, &protocol.SecurityKey{}); err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.CmdOutSend || msg.Ack != protocol.Request || msg.Register != 0x1b || !bytes.Equal(msg.Data, []byte{2, 3, 2, 0}) {
		t.Fatal("climate wire command changed")
	}
	// A status notification at the same address and an ACK for a different
	// register must not be mistaken for acknowledgement of the climate write.
	for _, m := range []*protocol.PhevMessage{
		{Type: protocol.CmdInResp, Ack: protocol.Request, Register: 0x1b, Data: []byte{2}},
		{Type: protocol.CmdInResp, Ack: protocol.Ack, Register: 0x17, Data: []byte{0}},
	} {
		if _, err := peer.Write(m.EncodeToBytes(&protocol.SecurityKey{})); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-result:
		t.Fatalf("completed without matching ACK: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	ack := &protocol.PhevMessage{Type: protocol.CmdInResp, Ack: protocol.Ack, Register: 0x1b, Data: []byte{0}}
	if _, err := peer.Write(ack.EncodeToBytes(&protocol.SecurityKey{})); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("climate write ACK was not delivered")
	}
}

func TestClimateWriteRecoversEncodingComplaint(t *testing.T) {
	c, _ := New()
	c.key.Update([]byte{0x5e, 0x0c, 0, 1, 0x3f, 0x58, 0xfa, 0xc7, 0x61, 0x9c, 0x83, 0xd8})
	expected := c.key.SKey(false)
	conn, peer := net.Pipe()
	c.conn = conn
	defer c.Close()
	defer peer.Close()
	go c.reader()
	go c.writer()
	go func() {
		for range c.Recv {
		}
	}()
	result := make(chan error, 1)
	go func() { result <- c.SetRegister(0x1b, []byte{2, 3, 2, 0}) }()
	peer.SetDeadline(time.Now().Add(2 * time.Second))
	first, retry := make([]byte, 9), make([]byte, 9)
	if _, err := io.ReadFull(peer, first); err != nil {
		t.Fatal(err)
	}
	bad := &protocol.PhevMessage{Type: protocol.CmdInBadEncoding, Data: []byte{expected}}
	if _, err := peer.Write(bad.EncodeToBytes(&protocol.SecurityKey{})); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(peer, retry); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, retry) {
		t.Fatalf("recovery changed climate command or expected XOR key: %x != %x", first, retry)
	}
	ack := &protocol.PhevMessage{Type: protocol.CmdInResp, Ack: protocol.Ack, Register: 0x1b, Data: []byte{0}}
	if _, err := peer.Write(ack.EncodeToBytes(&protocol.SecurityKey{})); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("recovered climate write did not complete")
	}
}

func TestClimateWriteACKSplitAcrossTCPReads(t *testing.T) {
	c, _ := New()
	conn, peer := net.Pipe()
	c.conn = conn
	defer c.Close()
	defer peer.Close()
	go c.reader()
	go c.writer()
	go func() {
		for range c.Recv {
		}
	}()
	result := make(chan error, 1)
	go func() { result <- c.SetRegister(0x1b, []byte{1, 0, 0, 0}) }()
	peer.SetDeadline(time.Now().Add(2 * time.Second))
	request := make([]byte, 9)
	if _, err := io.ReadFull(peer, request); err != nil {
		t.Fatal(err)
	}
	ack := &protocol.PhevMessage{Type: protocol.CmdInResp, Ack: protocol.Ack, Register: 0x1b, Data: []byte{0}}
	// TCP is a stream: even this six-byte ACK may arrive in separate reads.
	for _, b := range ack.EncodeToBytes(&protocol.SecurityKey{}) {
		if _, err := peer.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("valid climate ACK was lost when TCP split it across reads")
	}
}
