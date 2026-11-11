# ChargeNow / "ChargeOnTimer" evidence — phev-link 2.0.623

Scope: static review of decoded smali under
`/workspace/phev-research/decompiled/phev-link-2.0.623/smali/io/github/anwo91/phevlink/`.
No source changes, no device/network access.

## Naming / model classes

- Exact class present: `CarCommand$ChargeNow` (`core/model/CarCommand$ChargeNow.smali:1`).
- There is **no** `CarCommand$ChargeOnTimer` class in this build. `rg` over the app package
  finds only `ChargeNow` and `ResumeChargeTimer`/`SetChargeTimer`.
  The functional "charge on timer" command is **`CarCommand$ResumeChargeTimer`**
  (`core/model/CarCommand$ResumeChargeTimer.smali:1`), not `ChargeOnTimer`.
- Relevant command inventory: `core/model/CarCommand.smali:6-28`.

## Exact write payloads

`RegisterWrite` fields are `(register, data, pauseAfterMs, awaitState, awaitMs,
unconfirmedMessage, explainUnconfirmed)` (`core/model/RegisterWrite.smali:35-77`).

### ChargeNow — immediate override

`core/model/CarCommand$ChargeNow.smali:405-502`:
- `new byte[]{1}` at lines 442-453.
- `RegisterWrite(register=0x17, data=0x01, pause=0, awaitState=null, awaitMs=0,
  unconfirmed=null, explain=null)` at lines 460-489.
- Register constant: `WRITE_CHARGE_TIMER_CANCEL = 0x17`
  (`core/protocol/Reg.smali:83`); the read alias is `READ_CHARGE_TIMER_STATE = 0x17`
  (`core/protocol/Reg.smali:27`).
- Preconditions: requires fresh register `0x1e` and refuses only when
  `CarState.plug == UNPLUGGED` (`CarCommand$ChargeNow.smali:227-315`;
  required set `{0x1e}` at lines 317-360).
- No model-year branch in `toWrites`: `modelYear` is null-checked but unused
  (`CarCommand$ChargeNow.smali:419-433`, write at 438-502).
- No confirmation wait (`awaitState` null; `getConfirmMs()` inherited/0,
  `CarCommand$ChargeNow.smali:141-169`).

Interpretation: `ChargeNow` does **not** rewrite a schedule. It writes one byte
`0x01` to the timer state/override register `0x17`. It is a one-shot
"charge now / cancel waiting timer" override.

### ResumeChargeTimer — back to timer

`core/model/CarCommand$ResumeChargeTimer.smali:362-459`:
- `new byte[]{2}` at lines 395-411.
- `RegisterWrite(register=0x17, data=0x02, ...)` at lines 425-446.
- No model-year branch and no plug precondition (`requiredRegisters`/`refusal` are
  inherited, lines 225-332).

Interpretation: this is the inverse override, restoring timer control with
`0x17 = 0x02`. Again it does not rewrite the schedule.

### SetChargeTimer — real schedule write (MY14 only)

`core/model/CarCommand$SetChargeTimer.smali`:
- Rejects MY18 and MY24 through `CarCommandKt.isMy18Family`
  (`CarCommand$SetChargeTimer.smali:927-952`; helper
  `core/model/CarCommandKt.smali:143-178`). Message:
  "Setting the charge timer works on 2014–2017 cars for now"
  (`CarCommand$SetChargeTimer.smali:31`).
- Reads `0x1b` (timer state B), `0x20` (presets), `0x0e` (full-charge days) when
  an expected program is supplied (`CarCommand$SetChargeTimer.smali:1180-1276`).
- First write: `RegisterWrite(register=0x01,
  data=ChargeTimerCodec.encodeMy14(program, stateByte), awaitState=<compare timer>,
  awaitMs=0x2710, unconfirmed="The car did not confirm ...")`
  (`CarCommand$SetChargeTimer.smali:1609-1671`). Register constant
  `WRITE_CHARGE_TIMER_MY14 = 0x01` (`core/protocol/Reg.smali:85`).
- `encodeMy14` emits `0x1c` (28) bytes; state byte is placed at offset 11
  (`core/model/ChargeTimerCodec.smali:522-547`, `978-987`).
- Conditional `0x17` sync writes:
  - active program and current `0x1b` low nibble `< 2` -> `0x17 = stateByte`
    (`CarCommand$SetChargeTimer.smali:1729-1799`);
  - inactive program and current low nibble `>= 2` -> `0x17 = 0x01`
    (`CarCommand$SetChargeTimer.smali:1805-1875`).
- `stateByte` semantics (`core/model/ChargeTimerCodec.smali:1419-1496`):
  inactive -> `1`; active -> `2`, unless the car's current low nibble is already
  `>=2`, in which case the current byte is preserved.

Interpretation: the **true schedule write** is register `0x01`, 28 bytes. The
`0x17` writes here only reconcile the timer state/override bit; they are not the
schedule itself. `0x17=1` and `0x17=2` are the override values used by
`ChargeNow` and `ResumeChargeTimer`.

## Connection path: exact packet

- `PhevSession.enqueue` calls `command.toWrites(modelYear, state)` and stores the
  resulting `List<RegisterWrite>` in `PhevSession$Running`
  (`core/session/PhevSession.smali:16120-16210`).
- Each step reads `Running.write.register` / `.data` and builds
  `Packet(type=0xf6, ack, register, data)`:
  `core/session/PhevSession.smali:15220-15250`.
  `SEND = 0xf6` (`core/protocol/PacketType.smali:21`).
- Plain wire form is `[type, length, ack, register, data..., checksum]`
  (`core/protocol/Packet.smali:681-751`), then XOR-encoded
  (`core/protocol/Packet.smali:249-266`).
- Therefore, on the wire:
  - ChargeNow: register `0x17`, data `01`;
  - ResumeChargeTimer: register `0x17`, data `02`;
  - SetChargeTimer: register `0x01`, data = 28-byte MY14 program, optionally
    followed by register `0x17` data `02` or `01`.

## Status parsing

There are two distinct status concepts; do not conflate them.

1. **Timer active/waiting state from register `0x1b`**
   - `ScheduleDecoder.chargeTimerOn` reads register `0x1b`, takes first byte,
     `& 0x0f`, and returns true when `>= 2`
     (`core/model/ScheduleDecoder.smali:1120-1190`).
   - This matches the `stateByte` encoding above and is what the UI uses for the
     timer-on state (`ui/home/HomeScreenKt.smali:106512`, `125972`).
   - `READ_CHARGE_TIMER_STATE_B = 0x1b` (`core/protocol/Reg.smali:29`).

2. **CarState charge-wait/error flags from register `0x1e`**
   - `CarState` fields: `chargeWaiting` and `chargeTimerError`
     (`core/model/CarState.smali:29-33`).
   - `RegisterDecoder.apply` switch covers `0x1c`..`0x1f`
     (`core/model/RegisterDecoder.smali:1475`, table `6693-6699`).
   - For register `0x1e` (`pswitch_1`, register 30), second byte value:
     `== 1` -> `chargeWaiting = true`; `== 3` -> `chargeTimerError = true`
     (`core/model/RegisterDecoder.smali:1895-1933`).
   - The same handler derives plug state from bytes 0/1
     (`core/model/RegisterDecoder.smali:1761-1895`).
   - `READ_CHARGE_PLUG = 0x1e` (`core/protocol/Reg.smali:17`).
   - These flags are serialized/pushed (`core/json/Codecs.smali:2063-2098`,
     `core/push/PushMessage.smali:102`) but are not the `0x1b` timer-active bit.

3. **Register `0x1f` is charge status, not timer state**
   - `READ_CHARGE_STATUS = 0x1f` (`core/protocol/Reg.smali:21`).
   - `pswitch_0` (register `0x1f`) decodes `charging`, remaining charge minutes,
     and plug state; it does not set the timer-wait flags
     (`core/model/RegisterDecoder.smali:1489-1757`).
   - `chargeKind` is separately derived from `0x0d`/`0x0c` and `0x1e`
     (`core/model/RegisterDecoder.smali:216-525`).

## Model-year behavior

- Model year is derived from start type: `0x4e -> MY14`, `0x5e -> MY18`,
  `0x6e -> MY24` (`core/protocol/ModelYear$Companion.smali:38-100`;
  start constants `core/protocol/PacketType.smali:23-33`).
- `ChargeNow` and `ResumeChargeTimer` have **no model-year branching**; they emit
  `0x17=01` / `0x17=02` for every model year (the `modelYear` parameter is only
  null-checked).
- `SetChargeTimer` is the only year-gated command: MY18/MY24 are explicitly
  refused; MY14 (and, by this predicate, UNKNOWN) is allowed to use the MY14
  `0x01` schedule encoding (`CarCommand$SetChargeTimer.smali:927-952`;
  `CarCommandKt.smali:143-178`).

## Bottom line

- `ChargeNow` = one-byte temporary override `reg 0x17 = 0x01` (immediate charge /
  cancel waiting timer), with `0x1e` fresh-plug guard. No schedule rewrite, no
  confirmation wait, no year gate.
- "ChargeOnTimer" does not exist by that name; the matching third-party command is
  `ResumeChargeTimer` = `reg 0x17 = 0x02`, restoring timer control.
- The actual timer schedule is written only by `SetChargeTimer`: `reg 0x01`,
  28 bytes, MY14 only, with optional state reconciliation via `0x17`.
- Timer-active status is `reg 0x1b` first byte low nibble `>= 2`
  (`ScheduleDecoder.chargeTimerOn`); `CarState.chargeWaiting` /
  `chargeTimerError` come from `reg 0x1e` second byte values `1` / `3`;
  `reg 0x1f` is charging status, not timer status.

Confidence: high on class payloads, registers, model-year gates, and packet path;
medium-high on semantic labels (`0x17=1` vs `2`) because they are inferred from
the implementation's own constants and DevScript wording
(`core/dev/DevScripts.smali:805-878`), not from an official protocol spec.
