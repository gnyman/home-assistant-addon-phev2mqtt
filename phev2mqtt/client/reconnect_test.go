package client

import (
	"context"
	"errors"
	"github.com/buxtronix/phev2mqtt/protocol"
	"io"
	"net"
	"testing"
	"time"
)

func TestTrafficClosesCleanly(t *testing.T) {
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
	c, _ := New(AddressOption(l.Addr().String()))
	if err = c.Connect(); err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	peer := <-accepted
	defer peer.Close()
	drained := make(chan struct{})
	go func() {
		for range c.Recv {
		}
		close(drained)
	}()
	frame := protocol.NewPingResponseMessage(10).EncodeToBytes(&protocol.SecurityKey{})
	for i := 0; i < 35; i++ {
		if _, err = peer.Write(frame); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.Close()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("reader stuck")
	}
}

func TestBlockedSendUnblocksOnClose(t *testing.T) {
	c, _ := New()
	defer c.Close()
	for i := 0; i < cap(c.Send); i++ {
		c.SendMessage(protocol.NewPingRequestMessage(byte(i)))
	}
	result := make(chan error, 1)
	go func() { result <- c.SendMessage(protocol.NewPingRequestMessage(9)) }()
	c.Close()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("queued send accepted after closing full queue")
		}
	case <-time.After(time.Second):
		t.Fatal("full send queue remained blocked")
	}
}

func TestCloseUnblocksFullReceiveQueue(t *testing.T) {
	c, _ := New()
	defer c.Close()
	conn, peer := net.Pipe()
	c.conn = conn
	defer peer.Close()
	exited := make(chan struct{})
	go func() { c.reader(); close(exited) }()
	frame := protocol.NewPingResponseMessage(10).EncodeToBytes(&protocol.SecurityKey{})
	data := []byte{}
	for i := 0; i < cap(c.Recv)+2; i++ {
		data = append(data, frame...)
	}
	if _, err := peer.Write(data); err != nil {
		t.Fatal(err)
	}
	c.Close()
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("reader blocked on undrained Recv")
	}
}

func TestClosedClientRejectsNewListenersAndStart(t *testing.T) {
	c, _ := New()
	c.Close()
	listener := c.AddListener()
	select {
	case _, ok := <-listener.C:
		if ok {
			t.Fatal("expected closed listener")
		}
	case <-time.After(time.Second):
		t.Fatal("new listener after Close remained open")
	}
	if err := c.Start(); err == nil {
		t.Fatal("closed client started")
	}
}

func TestDialHasDeadline(t *testing.T) {
	c, _ := New()
	defer c.Close()
	if c.dialTimeout != 10*time.Second {
		t.Fatalf("default timeout=%v", c.dialTimeout)
	}
	c.dialTimeout = 20 * time.Millisecond
	c.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if _, ok := ctx.Deadline(); !ok {
			return nil, errors.New("dial has no deadline")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	result := make(chan error, 1)
	go func() { result <- c.Connect() }()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("dial error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial did not respect timeout")
	}
}

func TestCloseCancelsDial(t *testing.T) {
	c, _ := New()
	defer c.Close()
	dialing := make(chan struct{})
	c.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		close(dialing)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	result := make(chan error, 1)
	go func() { result <- c.Connect() }()
	<-dialing
	c.Close()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dial error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel dial")
	}
}

func TestLateDialSuccessDoesNotLeakSocket(t *testing.T) {
	c, _ := New()
	defer c.Close()
	conn, peer := net.Pipe()
	defer peer.Close()
	dialing, release := make(chan struct{}), make(chan struct{})
	c.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		close(dialing)
		<-release
		return conn, nil
	}
	result := make(chan error, 1)
	go func() { result <- c.Connect() }()
	<-dialing
	c.Close()
	close(release)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("closed client accepted dial result")
		}
	case <-time.After(time.Second):
		t.Fatal("late dial stuck")
	}
	peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("late socket did not close: %v", err)
	}
}

func TestRepeatedStartMessagesDoNotBlockShutdown(t *testing.T) {
	c, _ := New()
	defer c.Close()
	listener := c.AddListener()
	go c.manage(listener)
	for i := 0; i < 3; i++ {
		listener.Send(&protocol.PhevMessage{Type: protocol.CmdInMy18StartReq})
	}
	for i := 0; i < 3; i++ {
		select {
		case <-c.Send:
		case <-time.After(time.Second):
			t.Fatal("manager stopped acknowledging start messages")
		}
	}
	c.Close()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-c.started:
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("manager leaked on repeated handshake")
		}
	}
}
