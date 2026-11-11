package protocol

import (
	"bytes"
	"testing"
)

func TestStreamDecoderEverySplit(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		sender := &SecurityKey{}
		if encrypted {
			sender.Update([]byte{0x5e, 0x0c, 0, 1, 0x3f, 0x58, 0xfa, 0xc7, 0x61, 0x9c, 0x83, 0xd8})
		}
		frames := [][]byte{
			(&PhevMessage{Type: CmdInResp, Ack: Ack, Register: 0x1b, Data: []byte{0}}).EncodeToBytes(sender),
			(&PhevMessage{Type: CmdInResp, Ack: Request, Register: 0x1a, Data: []byte{0, 1, 2, 0, 0}}).EncodeToBytes(sender),
			NewPingResponseMessage(10).EncodeToBytes(sender),
			(&PhevMessage{Type: CmdInBadEncoding, Data: []byte{0x45}}).EncodeToBytes(sender),
		}
		wire := bytes.Join(frames, nil)
		for split := 0; split <= len(wire); split++ {
			d := NewStreamDecoder(&SecurityKey{})
			got := append(d.Feed(wire[:split]), d.Feed(wire[split:])...)
			if len(got) != len(frames) {
				t.Fatalf("encrypted=%t split=%d decoded %d packets, want %d", encrypted, split, len(got), len(frames))
			}
			for i, msg := range got {
				if !bytes.Equal(msg.OriginalXored, frames[i]) {
					t.Fatalf("split=%d frame=%d changed bytes", split, i)
				}
			}
		}
		d := NewStreamDecoder(&SecurityKey{})
		var got []*PhevMessage
		for _, b := range wire {
			got = append(got, d.Feed([]byte{b})...)
		}
		if len(got) != len(frames) {
			t.Fatalf("bytewise: decoded %d packets, want %d", len(got), len(frames))
		}
		if len(d.pending) != 0 {
			t.Fatal("complete stream left buffered bytes")
		}
	}
}

func TestStreamDecoderRetainsTrailingPartialAndRecoversNoise(t *testing.T) {
	frame := (&PhevMessage{Type: CmdInResp, Ack: Ack, Register: 0x1b, Data: []byte{0}}).EncodeToBytes(&SecurityKey{})
	d := NewStreamDecoder(&SecurityKey{})
	first := append(append([]byte{0xaa, 0xaa, 0xaa, 0xaa}, frame...), frame[:3]...)
	got := d.Feed(first)
	if len(got) != 1 {
		t.Fatal("did not recover packet after noise")
	}
	saved := append([]byte(nil), got[0].OriginalXored...)
	got = d.Feed(frame[3:])
	if len(got) != 1 || !bytes.Equal(got[0].OriginalXored, saved) {
		t.Fatal("trailing partial frame lost or mutated")
	}
	if got := d.Feed(nil); len(got) != 0 {
		t.Fatal("empty feed repeated packet")
	}
}

func TestStreamDecoderBoundsIncompleteBuffer(t *testing.T) {
	d := NewStreamDecoder(&SecurityKey{})
	for i := 0; i < 10000; i++ {
		d.Feed([]byte{byte(i % 256)})
		if len(d.pending) > 254 {
			t.Fatal("stream buffer grew beyond maximum incomplete packet size")
		}
	}
}
