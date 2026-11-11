package protocol

// StreamDecoder preserves incomplete packets across TCP reads. TCP read
// boundaries need not coincide with protocol packet boundaries.
type StreamDecoder struct {
	key     *SecurityKey
	pending []byte
}

func NewStreamDecoder(key *SecurityKey) *StreamDecoder { return &StreamDecoder{key: key} }

func (d *StreamDecoder) Feed(chunk []byte) []*PhevMessage {
	d.pending = append(d.pending, chunk...)
	var messages []*PhevMessage
	for len(d.pending) >= 4 {
		incomplete, consumed := false, false
		// Byte 2 decodes to either Request (0) or Ack (1), giving two XOR candidates.
		for _, ack := range []byte{Request, Ack} {
			xor := d.pending[2] ^ ack
			typ := d.pending[0] ^ xor
			if _, known := messageStr[typ]; !known && typ != CmdInBadEncoding && typ != CmdInUnkn3 && typ != CmdInUnkn4 {
				continue
			}
			size := int(d.pending[1]^xor) + 2
			// PhevMessage.Length is a byte; reject unsupported or malformed lengths.
			if size < 5 || size > 255 {
				continue
			}
			if len(d.pending) < size {
				incomplete = true
				continue
			}
			raw := d.pending[:size]
			plain := XorMessageWith(raw, xor)
			if !ValidateChecksum(plain) {
				continue
			}
			// Own the bytes: queued messages must survive subsequent buffer appends.
			raw = append([]byte(nil), raw...)
			msg := &PhevMessage{}
			if err := msg.DecodeFromBytes(raw, d.key); err != nil {
				continue
			}
			messages = append(messages, msg)
			d.pending = d.pending[size:]
			consumed = true
			break
		}
		if consumed {
			continue
		}
		if incomplete {
			break
		}
		// Skip corrupt bytes to recover the next valid packet, retaining any tail.
		d.pending = d.pending[1:]
	}
	return messages
}
