# Experimental charge timer override

This add-on exposes an experimental Home Assistant switch for the PHEV charge
timer override. The command mapping comes from third-party static evidence. Resume-timer
(`0x17=02`) has also been acknowledged and verified by status read-back on the
user’s MY20. The opposite command remains supported by static evidence only.

## Provenance

- Official APK: `com.inventec.iMobile2`, A.3.1.6 (versionCode 141), downloaded from
  `https://d.apkpure.net/b/APK/com.inventec.iMobile2?versionCode=141`
- APK SHA-256: `dbb8ce98d4a683c310106bde5465e791fcd04168c3904ee1b019aab968afb5aa`
- PHEV Link: [2.0.623 release](https://github.com/Anwo91/phev-remote-plus/releases/tag/v2.0.623)
- PHEV Link SHA-256: `22458594fce17bd98bc63f40a5ef1ef9500f581bb644971e06075aa373317d8a`
- Vendored upstream phev2mqtt commit: `8f423405eef2f833a6a90a93a577624c5875a24c`

The protocol evidence below is quoted from
[chargenow-evidence.md](chargenow-evidence.md), a static review of the decoded
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
- `core/model/RegisterDecoder.smali:1895-1933`: register `0x1e` byte 1
  indicates timer waiting (`1`) or timer error (`3`). Register `0x1f`, decoded
  at lines 1489-1757, is charging status and is not used as override state.

The evidence also shows that the real MY14 schedule write is register `0x01`
with a 28-byte payload. The override feature adds no schedule read requests, schedule writes, or clock
changes. Existing generic register access remains available.

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
state is never used to infer the override state. Until a fresh valid `0x1b`
notification arrives, the discovery entity stays unavailable. Unknown data and
car or MQTT reconnections invalidate its status, preventing stale ON/OFF display.

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
- `availability_mode`: `all`, with both `~/available` and
  `~/charge/override/available` required to be `online`
- `icon`: `mdi:timer-sync-outline`

## Encoding recovery and live verification

On 2026-11-11, a short direct capture from the user’s MY20 returned `0x17=01`,
`0x1b=01`, plug status `0200`, and charging status `01a500`. The original client
reported repeated `0xbb` encoding complaints. A single authorized `0x17=02`
write received no acknowledgement and register `0x17` remained `01`.

PHEV Link handles this complaint by finding the car’s expected XOR byte in its
key table and resetting its send index (`SecurityKey.smali:402-460`, invoked
from `PhevSession.smali:10150-10162`). The vendored client now applies that same
resynchronization **before** notifying command listeners. Key-table operations
are synchronized between its reader and writer. It does not turn timeouts into
success or suppress errors.

After that fix, another single authorized `0x17=02` request received a
`REGISTER SETACK` for `0x17`, then a notification `0x1b=02`. Plug status changed
to `0201`, matching timer waiting. The connection was closed immediately after
confirmation. The car was left with timer control enabled / override disabled.
No schedule or clock commands were sent. Some recoverable key complaints still
occurred; this is not a claim that every transport problem is solved.

## Limitations

- The switch reports the inverse of PHEV Link’s timer-active status. A disabled
  timer without configured schedules may be indistinguishable from an override;
  it is not a separately established persistent override flag.
- Only resume-timer / override OFF was verified on this MY20. Charge-now / ON
  remains supported by PHEV Link code rather than a live toggle in this session.
- The 10-second write-acknowledgement timeout remains in force. A missing ACK is
  still an error; command sending alone never updates the reported state.
- The mapping is not universally validated across model years. The official
  app was acquired and decompiled, but the command evidence cited here is from
  PHEV Link, not a completed official-app trace.
- HA discovery was checked offline; the add-on was not deployed and no MQTT
  broker or Home Assistant was contacted. A full Docker image build was not run.
