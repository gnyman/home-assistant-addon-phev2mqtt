# Experimental charge timer override

This add-on exposes an experimental Home Assistant switch for the PHEV charge
timer override. It is implemented from third-party static evidence and has not
been verified against a live MY20 vehicle.

## Provenance

- APK SHA-256: `dbb8ce98d4a683c310106bde5465e791fcd04168c3904ee1b019aab968afb5aa`
- PHEV Link SHA-256: `22458594fce17bd98bc63f40a5ef1ef9500f581bb644971e06075aa373317d8a`
- Vendored upstream phev2mqtt commit: `8f423405eef2f833a6a90a93a577624c5875a24c`

The protocol evidence below is quoted from
`/tmp/phev-review/chargenow-evidence.md`, a static review of the decoded
PHEV Link 2.0.623 smali. It is third-party reverse-engineering evidence, not an
official Mitsubishi protocol specification.

## Cited evidence

- `core/model/CarCommand$ChargeNow.smali:405-502`: ChargeNow builds a
  `RegisterWrite(register=0x17, data=0x01)`.
- `core/model/CarCommand$ResumeChargeTimer.smali:362-459`: ResumeChargeTimer
  builds a `RegisterWrite(register=0x17, data=0x02)`.
- `core/model/ScheduleDecoder.smali:1120-1190`: `chargeTimerOn` reads the first
  byte of register `0x1b`, takes `& 0x0f`, and treats low nibble `>= 2` as the
  active timer state.
- `core/protocol/Reg.smali:27,83,85`: `READ_CHARGE_TIMER_STATE = 0x17`,
  `WRITE_CHARGE_TIMER_CANCEL = 0x17`, and `WRITE_CHARGE_TIMER_MY14 = 0x01`.
- `core/session/PhevSession.smali:15220-15250` and
  `core/protocol/Packet.smali:681-751`: a register write becomes a packet with
  type `0xf6`, the write register, and the write data.
- `core/model/RegisterDecoder.smali:1895-1933` / register `0x1f`: charging
  status is distinct from the timer/override state. Register `0x1f` is not used
  as override state.

The evidence also shows that the real MY14 schedule write is register `0x01`
with a 28-byte payload. This add-on does not implement schedule writes, schedule
reads, or clock changes.

## Topic mapping

| Topic | Direction | Behavior |
|---|---|---|
| `<prefix>/charge/override` | published | `on`, `off`, or `unknown`, derived only from incoming register `0x1b` notifications |
| `<prefix>/register/1b` | published | Existing raw register `0x1b` hex value, preserved |
| `<prefix>/set/charge/override` | subscribed | Accepts `on` or `off` (case-insensitive, surrounding whitespace trimmed) |
| `<prefix>/set/cancelchargetimer` | subscribed | Legacy alias for the same validated `on`/`off` behavior |

`<prefix>/charge/charging` remains the charging status published from register
`0x1f`; it is not the override state.

## 0x17 payload mapping

| Command | Register | Data | Writes |
|---|---|---|---|
| On | `0x17` | `01` | Exactly one `SetRegister(0x17, []byte{0x01})` |
| Off | `0x17` | `02` | Exactly one `SetRegister(0x17, []byte{0x02})` |

There is no second write and no `0x11` write. Any payload other than `on` or
`off` is rejected before touching the client, so it produces zero register
writes. Values previously accepted by the legacy topic such as arbitrary text,
`1`, `2`, `01`, `0x01`, or an empty payload are now rejected.

## 0x1b notification state mapping

State is published only when a register `0x1b` notification arrives through the
normal notification path. Commands do not set optimistic state, and charging
state is never used to infer the override state. Until a real `0x1b`
notification arrives, the discovery entity remains in the unknown state.

| First data byte low nibble | Published state |
|---|---|
| `1` | `on` |
| `>= 2` | `off` |
| `0` | `unknown` |
| Empty or absent data | `unknown` |

The high nibble is ignored, so `0x11` maps to `on` and `0x12` maps to `off`.

## Home Assistant discovery

The existing switch discovery topic and object id are retained for migration:
`homeassistant/switch/<vin>_cancel_charge_timer/config` with
`unique_id = <vin>_cancel_charge_timer`. Existing Home Assistant entities are
therefore updated rather than duplicated. The old incorrect state topic
`~/battery/charging` and old command topic `~/set/cancelchargetimer` are removed
from discovery.

The published switch configuration uses:

- `state_topic`: `~/charge/override`
- `command_topic`: `~/set/charge/override`
- `payload_on`: `on`
- `payload_off`: `off`
- `avty_t`: `~/available`
- `icon`: `mdi:timer-sync-outline`

## Limitations

- **Experimental.** The behavior is inferred from third-party PHEV Link
  semantics, not from an official protocol specification.
- **No live MY20 verification.** No vehicle was contacted while preparing this
  patch.
- The existing `SetRegister` 10-second ACK timeout, retry logic, and transport
  are unchanged. If the vehicle does not ACK a `0x17` write, the same
  charge-now ACK timeout may persist.
- The full official ACK trace remains unresolved. Errors remain visible in the
  add-on log; this patch does not claim the MY20 timeout issue is resolved.
- Do not treat this as universal protocol validation.
- No schedule read requests, schedule writes, or clock changes are implemented.
