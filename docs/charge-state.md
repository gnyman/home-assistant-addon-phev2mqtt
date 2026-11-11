# Charging state

Add-on 1.7 publishes `<prefix>/charge/state` and discovers a **Charge State**
MQTT sensor in Home Assistant. This is a read-only feature: it sends no car
commands and requests no new registers.

| State | Evidence |
|---|---|
| `error` | Register `0x1e` byte 1 is `3` (charge-timer error) |
| `waiting` | Register `0x1e` byte 1 is `1` (waiting for timer) |
| `unplugged` | Register `0x1e` is `0000` |
| `charging` | Connected, no waiting/error flag, and register `0x1f` byte 0 is `1` |
| `idle` | Connected, no waiting/error flag, and register `0x1f` byte 0 is `0` |
| `unknown` | Required data is missing, malformed, or has an unsupported status |

Error/waiting flags take precedence over the charging bit. The captured MY20
notifications after resuming timer control were `0x1e=0201` and `0x1f=01a500`:
the new sensor correctly reports `waiting` even though the separate charging
bit is still set. The existing `/charge/charging` topic continues to report
that bit as `on`, preserving its binary sensor and existing automations.

The waiting/error mapping is from PHEV Link 2.0.623's
`core/model/RegisterDecoder.smali:1895-1933`; see
[the recorded protocol evidence](chargenow-evidence.md#status-parsing) and
[artifact provenance](charge-override.md#provenance). The charging bit is decoded
by phev2mqtt's existing `RegisterChargeStatus` implementation. `idle` is a
description of connected/not-charging state, not a claim that charging is
complete or the battery is full. No other charging errors are inferred.

State messages are not retained. Freshness is published as retained
`online`/`offline` on `<prefix>/charge/state/available`; HA requires both this
topic and `<prefix>/available` to be online. Unknown data and car/MQTT
reconnections make the sensor unavailable, clearing the cached input registers.
A change in the plug register also invalidates the previous charging bit, so
replugging or leaving the waiting state requires fresh charging data before
reporting `charging` or `idle`.

The new decoding, MQTT publications, discovery, and reconnect behavior were
tested offline with fake MQTT and captured bytes. No new car connection,
MQTT broker connection, or Home Assistant deployment was performed for this
change.
