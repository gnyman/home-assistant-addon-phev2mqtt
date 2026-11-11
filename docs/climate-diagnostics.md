# Climate diagnostics

Set these app options and restart:

```yaml
debug: true
log_pings: false
```

Debug mode keeps the bridge running. It logs register traffic and encoding
recovery, while filtering routine ping requests and responses. `log_pings: true`
includes those keepalives when needed. The same standalone flags are
`--verbosity debug --log_pings=false`.

For a climate command, inspect the sequence:

- `Register write requested` and `REGISTER SET`: intended register and payload.
- `BAD ENCODING`, `PHEV_SEND_KEY_RESYNC`, and `Retrying register write`: transport
  rejected encoding and attempted to recover the send-key position.
- `REGISTER SETACK` and `Register write acknowledged`: the matching write ACK.
- `REGISTER NTFY`: a status notification, which is not a write ACK, even when
  the numeric register address is the same.

A timeout reports the number of encoding retries. It does not claim that a
command took effect, and a command is not acknowledged optimistically.

The existing MY18/MY24 command for windscreen with duration `30` remains
`0x1b = 02 03 02 00`. Offline in-memory socket tests cover this wire payload,
matching ACK delivery, ignoring unrelated ACKs and status notifications, and
encoding recovery. These tests do not prove that a real vehicle accepts the
command in its current state.

Pre-AC termination status arrives in register `0x10` and is exposed as
`terminated` on the climate state topic. A selected mode such as `windscreen`
can remain visible while AC is off. A termination status alone does not prove
why a particular command timed out. This release adds no automatic termination
reset or extra climate writes.

## TCP framing

Version 1.10.3 fixes a confirmed receiver bug: it treated each TCP read as a
complete packet buffer and could discard a write ACK split across reads. The
receiver now retains incomplete bytes and decodes complete packets in order.
The regression test delivers a valid climate OFF ACK one byte at a time; it
fails before the fix and passes afterwards. Coalesced and encrypted packet
streams are also tested across all split positions.

This is a possible explanation for an intermittent missing-ACK timeout, not
proof of the cause of every vehicle timeout. No arbitrary startup delay or
automatic climate retry/reset is added. The message labelled `START RESP`
(0x2f) is documented as a keepalive and is not used as a readiness gate.
