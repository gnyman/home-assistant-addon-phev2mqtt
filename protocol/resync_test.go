package protocol

import (
	"bytes"
	"testing"
)

func TestResyncSendSequence(t *testing.T) {
	key := &SecurityKey{}
	key.Update([]byte{0x5e, 0x0c, 0, 1, 0x3f, 0x58, 0xfa, 0xc7, 0x61, 0x9c, 0x83, 0xd8})
	expected := key.SKey(true)
	next := key.SKey(false)
	if expected == next {
		t.Fatal("test requires distinct consecutive keys")
	}
	if !key.ResyncSend(expected) {
		t.Fatal("expected key absent")
	}
	msg := &PhevMessage{Type: CmdOutSend, Ack: Request, Register: 0x17, Data: []byte{2}}
	encoded := msg.EncodeToBytes(key)
	plain := XorMessageWith(encoded, expected)
	if !bytes.Equal(plain, []byte{0xf6, 4, 0, 0x17, 2, 0x13}) {
		t.Fatalf("wrong retry bytes: %x", plain)
	}
	if got := key.SKey(false); got != next {
		t.Fatalf("next key %02x, want %02x", got, next)
	}
}

func TestResyncSendZeroAndWrap(t *testing.T) {
	key := &SecurityKey{keyMap: make([]byte, 256)}
	for i := range key.keyMap {
		key.keyMap[i] = byte(i)
	}
	if !key.ResyncSend(0xff) || key.SKey(true) != 0xff || key.SKey(false) != 0 {
		t.Fatal("resync/wrap failed")
	}
	if !key.ResyncSend(0) || key.SKey(true) != 0 || key.SKey(false) != 1 {
		t.Fatal("zero key resync failed")
	}
	if (&SecurityKey{}).ResyncSend(0) {
		t.Fatal("empty key map must not resync")
	}
}
