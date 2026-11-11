// Package client implements a client for communicating with a Mitsubishi
// Outlander Phev.
package client

import (
	"context"
	"encoding/hex"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"

	"github.com/buxtronix/phev2mqtt/protocol"
)

const DefaultAddress = "192.168.8.46:8080"
const DefaultDialTimeout = 10 * time.Second

// A Listener is for communicating messages from the vehicle to
// interested clients.
type Listener struct {
	// C has received messages.
	C      chan *protocol.PhevMessage
	mu     sync.Mutex
	stop   bool
	closed bool
}

func (l *Listener) Start() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stop = false
	l.closed = false
	l.C = make(chan *protocol.PhevMessage, 5)
}

func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stop = true
}

func (l *Listener) Send(m *protocol.PhevMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	select {
	case l.C <- m:
	default:
		log.Debug("%PHEV_RECV_LISTENER% message not sent")
	}
}

func (l *Listener) ProcessStop() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop && !l.closed {
		close(l.C)
		l.closed = true
		l.stop = false
		return true
	}
	return false
}

type ModelYear int64

const (
	ModelYearUnknown ModelYear = iota
	ModelYear14
	ModelYear18
	ModelYear24
)

// A Client is a TCP client to a Phev.
type Client struct {
	// Recv is a channel where incoming messages from the Phev are sent.
	Recv chan *protocol.PhevMessage
	// Send is a channel to send messages to the Phev.
	Send chan *protocol.PhevMessage

	// Settings are settings for the car.
	Settings *protocol.Settings

	listeners []*Listener
	lMu       sync.Mutex

	address     string
	conn        net.Conn
	connMu      sync.Mutex
	lastRx      time.Time
	lastRxMu    sync.Mutex
	started     chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	dialTimeout time.Duration
	dialContext func(context.Context, string, string) (net.Conn, error)

	logPings bool
	key      *protocol.SecurityKey

	// Keep track of the model year so we can use the correct registers
	ModelYear ModelYear

	done      chan struct{}
	closeOnce sync.Once
}

// isClosed reports whether Close has been called.
func (c *Client) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// An Option configures the client.
type Option func(c *Client)

// AddressOption configures the address to the Phev.
func AddressOption(address string) func(*Client) {
	return func(c *Client) {
		c.address = address
	}
}

// LogPingsOption includes routine keepalive packets in protocol logs.
func LogPingsOption(enabled bool) Option {
	return func(c *Client) { c.logPings = enabled }
}

// New returns a new client, not yet connected.
func New(opts ...Option) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cl := &Client{
		Recv:        make(chan *protocol.PhevMessage, 5),
		Send:        make(chan *protocol.PhevMessage, 5),
		Settings:    &protocol.Settings{},
		started:     make(chan struct{}, 2),
		listeners:   []*Listener{},
		address:     DefaultAddress,
		key:         &protocol.SecurityKey{},
		ModelYear:   ModelYearUnknown,
		done:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		dialTimeout: DefaultDialTimeout,
		dialContext: (&net.Dialer{}).DialContext,
	}
	for _, o := range opts {
		o(cl)
	}
	return cl, nil
}

// Create and return a new Listener.
func (c *Client) AddListener() *Listener {
	c.lMu.Lock()
	defer c.lMu.Unlock()
	l := &Listener{}
	l.Start()
	if c.isClosed() {
		l.Stop()
		l.ProcessStop()
		return l
	}
	c.listeners = append(c.listeners, l)
	return l
}

func (c *Client) RemoveListener(l *Listener) {
	newL := []*Listener{}
	c.lMu.Lock()
	defer c.lMu.Unlock()
	for _, lis := range c.listeners {
		if lis != l {
			newL = append(newL, lis)
		}
	}
	c.listeners = newL
}

// Close closes the client.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		c.cancel()
		c.connMu.Lock()
		if c.conn != nil {
			err = c.conn.Close()
		}
		c.connMu.Unlock()
		c.lMu.Lock()
		for _, l := range c.listeners {
			l.Stop()
			l.ProcessStop()
		}
		c.lMu.Unlock()
	})
	return err
}

// SendMessage queues a message for sending to the car. Unlike sending
// directly to the Send channel, it will not block if the client is closed.
func (c *Client) SendMessage(m *protocol.PhevMessage) error {
	if c.isClosed() {
		return fmt.Errorf("client closed")
	}
	select {
	case c.Send <- m:
		return nil
	case <-c.done:
		return fmt.Errorf("client closed")
	}
}

// Connect connects to the Phev.
func (c *Client) Connect() error {
	if c.isClosed() {
		return fmt.Errorf("client closed")
	}
	ctx, cancel := context.WithTimeout(c.ctx, c.dialTimeout)
	defer cancel()
	conn, err := c.dialContext(ctx, "tcp", c.address)
	if err != nil {
		return err
	}
	c.connMu.Lock()
	if c.isClosed() {
		c.connMu.Unlock()
		conn.Close()
		return fmt.Errorf("client closed")
	}
	c.conn = conn
	c.connMu.Unlock()
	log.Info("%PHEV_TCP_CONNECTED%")
	manager := c.AddListener()
	go c.manage(manager)
	go c.reader()
	go c.writer()
	go c.pinger()

	return nil
}

var startTimeout = 20 * time.Second

// Start waits for the client to start.
func (c *Client) Start() error {
	log.Debug("%%PHEV_START_AWAIT%%")
	startTimer := time.NewTimer(startTimeout)
	defer startTimer.Stop()
	for {
		select {
		case <-c.done:
			return fmt.Errorf("client closed")
		case _, ok := <-c.started:
			if !ok {
				log.Debug("%%PHEV_START_CLOSED%%")
				return fmt.Errorf("receiver closed before getting start request")
			}
			log.Debug("%%PHEV_START_DONE%%")
		case <-startTimer.C:
			log.Debug("%%PHEV_START_TIMEOUT%%")
			return fmt.Errorf("timed out waiting for start")
		}
		return nil
	}
}

// SetRegister sets a register on the car.
func (c *Client) SetRegister(register byte, value []byte) error {
	setRegister := func(xor byte) error {
		return c.SendMessage(&protocol.PhevMessage{
			Type:     protocol.CmdOutSend,
			Ack:      protocol.Request,
			Register: register,
			Data:     value,
			Xor:      xor,
		})
	}
	log.Debugf("Register write requested: reg 0x%02x data %s", register, hex.EncodeToString(value))
	encodingRetries := 0
	xor := byte(0)
	timer := time.After(10 * time.Second)
	l := c.AddListener()
	defer c.RemoveListener(l)
SETREG:
	if err := setRegister(xor); err != nil {
		return err
	}
	for {
		select {
		case <-c.done:
			return fmt.Errorf("client closed")
		case <-timer:
			return fmt.Errorf("timed out attempting to set register %02x: no write ACK within 10s (%d encoding retries)", register, encodingRetries)
		case msg, ok := <-l.C:
			if !ok {
				return fmt.Errorf("listener channel closed")
			}
			if msg.Type == protocol.CmdInBadEncoding {
				if len(msg.Data) == 0 {
					return fmt.Errorf("bad encoding response omitted expected key")
				}
				xor = msg.Data[0]
				encodingRetries++
				log.Debugf("Retrying register write: reg 0x%02x after encoding complaint, expected key %02x (retry %d)", register, xor, encodingRetries)
				goto SETREG
			}
			if msg.Type == protocol.CmdInResp && msg.Ack == protocol.Ack && msg.Register == register {
				log.Debugf("Register write acknowledged: reg 0x%02x (%d encoding retries)", register, encodingRetries)
				return nil
			}

		}
	}
}

func (c *Client) nextRecvMsg(deadline time.Time) (*protocol.PhevMessage, error) {
	timer := time.After(deadline.Sub(time.Now()))
	for {
		select {
		case <-timer:
			return nil, fmt.Errorf("timed out waiting for message")
		case m, ok := <-c.Recv:
			if !ok {
				return nil, fmt.Errorf("error: receive channel closed")
			}
			return m, nil
		}
	}
}

// Sends periodic pings to the car.
func (c *Client) pinger() {
	pingSeq := byte(0xa)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for t := range ticker.C {
		c.lastRxMu.Lock()
		lastRx := c.lastRx
		c.lastRxMu.Unlock()
		switch {
		case c.isClosed():
			return
		case t.Sub(lastRx) < 500*time.Millisecond:
			continue
		}
		if err := c.SendMessage(protocol.NewPingRequestMessage(pingSeq)); err != nil {
			return
		}
		pingSeq++
		if pingSeq > 0x63 {
			pingSeq = 0
		}
	}
}

// manages the connection, handling control messages.
func (c *Client) manage(ml *Listener) {
	defer close(c.started)
	defer log.Debug("%PHEV_MANAGER_END%%")
	defer ml.Stop()
	for m := range ml.C {
		switch m.Type {
		case protocol.CmdInResp:
			if m.Ack == protocol.Request && m.Register == protocol.SettingsRegister {
				c.Settings.FromRegister(m.Data)
			}
		case protocol.CmdInStartResp:
			if err := c.SendMessage(protocol.NewPingRequestMessage(0xa)); err != nil {
				return
			}
		case protocol.CmdInMy24StartReq:
			c.ModelYear = ModelYear24
			if err := c.SendMessage(&protocol.PhevMessage{
				Type:     protocol.CmdOutMy24StartResp,
				Register: 0x1,
				Ack:      protocol.Ack,
				Xor:      m.Xor,
				Data:     []byte{0x0},
			}); err != nil {
				return
			}
			log.Debug("%%PHEV_START24_RECV%%")
			select {
			case c.started <- struct{}{}:
			case <-c.done:
				return
			default:
			}
		case protocol.CmdInMy18StartReq:
			c.ModelYear = ModelYear18
			if err := c.SendMessage(&protocol.PhevMessage{
				Type:     protocol.CmdOutMy18StartResp,
				Register: 0x1,
				Ack:      protocol.Ack,
				Xor:      m.Xor,
				Data:     []byte{0x0},
			}); err != nil {
				return
			}
			log.Debug("%%PHEV_START18_RECV%%")
			select {
			case c.started <- struct{}{}:
			case <-c.done:
				return
			default:
			}
		case protocol.CmdInMy14StartReq:
			c.ModelYear = ModelYear14
			if err := c.SendMessage(&protocol.PhevMessage{
				Type:     protocol.CmdOutMy14StartResp,
				Register: 0x1,
				Ack:      protocol.Ack,
				Xor:      m.Xor,
				Data:     []byte{0x0},
			}); err != nil {
				return
			}
			log.Debug("%%PHEV_START14_RECV%%")
			select {
			case c.started <- struct{}{}:
			case <-c.done:
				return
			default:
			}
		}
	}
}

func (c *Client) reader() {
	decoder := protocol.NewStreamDecoder(c.key)
	defer func() {
		c.Close()
		close(c.Recv)
	}()
	for {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		data := make([]byte, 4096)
		n, err := c.conn.Read(data)
		if err != nil {
			if !c.isClosed() {
				log.Debug("%%PHEV_TCP_READER_ERROR%%: ", err)
			}
			log.Debug("%PHEV_TCP_READER_CLOSE%")
			return
		}
		c.lastRxMu.Lock()
		c.lastRx = time.Now()
		c.lastRxMu.Unlock()
		messages := decoder.Feed(data[:n])
		if len(messages) == 0 {
			log.Tracef("%%PHEV_TCP_RECV_BUFFERED%%: %s", hex.EncodeToString(data[:n]))
		}
		for _, m := range messages {
			if m.Type == protocol.CmdInBadEncoding && len(m.Data) > 0 {
				if c.key.ResyncSend(m.Data[0]) {
					log.Debugf("%%PHEV_SEND_KEY_RESYNC%% expected %02x", m.Data[0])
				}
			}
			c.logMessage("RECV", m, m.OriginalXored)
			c.lMu.Lock()
			for _, l := range c.listeners {
				l.Send(m)
			}
			c.lMu.Unlock()
			select {
			case c.Recv <- m:
			case <-c.done:
				return
			}
		}
	}
}

func (c *Client) writer() {
	for {
		select {
		case <-c.done:
			log.Debug("%PHEV_TCP_WRITER_CLOSE%")
			return
		case msg, ok := <-c.Send:
			if !ok {
				log.Debug("%PHEV_TCP_WRITER_CLOSE%")
				c.Close()
				return
			}
			msg.Xor = 0
			data := msg.EncodeToBytes(c.key)
			c.logMessage("SEND", msg, data)
			c.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			if _, err := c.conn.Write(data); err != nil {
				if !c.isClosed() {
					log.Errorf("%%PHEV_TCP_WRITER_ERROR%%: %v", err)
				}
				log.Debug("%PHEV_TCP_WRITER_CLOSE%")
				c.Close()
				return
			}
		}
	}
}

// Filter only routine keepalives; register writes, ACKs and errors stay visible.
func (c *Client) logMessage(direction string, msg *protocol.PhevMessage, raw []byte) {
	if !c.logPings && (msg.Type == protocol.CmdOutPingReq || msg.Type == protocol.CmdInPingResp) {
		return
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("%%PHEV_TCP_%s_MSG%%: [%02x] %s", direction, msg.Xor, msg.ShortForm())
	}
	log.Tracef("%%PHEV_TCP_%s_DATA%%: %s", direction, hex.EncodeToString(raw))
}
