# Home Assistant add-on: PHEV-DRAGONS

Source: https://github.com/gnyman/home-assistant-addon-phev2mqtt

Bridge: https://github.com/gnyman/phev2mqtt (vendored in `phev2mqtt/`).

Supported Home Assistant architectures: **aarch64** and **amd64**.

Version 1.10 uses prebuilt images from GHCR, so the Home Assistant device only
downloads and extracts a small image. The runtime contains a static bridge,
a small Home Assistant options launcher, and TLS certificates. It has no shell
or package manager. The build toolchain stays out of the runtime image.
See [image publishing](docs/images.md) for release steps and size measurements.

This requires that Home Assistant is already connected to the Outlander, you should be able to check this by ensuring that you can ping 192.168.8.46

Once installed as a local addon, you must configure the mqtt target and username/password before start and stop phev2mqtt from the GUI
For my docker install the host machine's address (which is also running the mqtt addon) is 172.30.32.1:1883 (I suspect this could be different depending on install)

PHEV-DRAGONS uses the `phev-dragons` slug and can be installed alongside the old local PHEV app. Keep only one bridge running at a time.

For a local installation, put these files in a directory called `phev-dragons` in the local addons.

If you ssh into the main os, this will be in /usr/share/hassio/addons/local/phev-dragons
If you ssh in using the terminal add on, this will be /addons/phev-dragons (or symblink /root/addons/phev-dragons)
If you connect to the hassio_supervisort docker container, this will be /data/addons/local/phev-dragons

More information about local addons at https://developers.home-assistant.io/docs/add-ons/tutorial/

With `debug: true`, the bridge keeps running and logs protocol messages, register
writes and ACKs, encoding errors, and key resynchronization. Routine PING traffic
is hidden by default; set `log_pings: true` to include it. Restart the app after
changing these options. There is no interactive shell in the minimal image;
execute `/opt/phev2mqtt` directly for diagnostics. WiFi is managed by the Home
Assistant host, so the standalone WiFi restart command is disabled.

## Reconnecting

Version 1.8 incorporates [upstream PR #62](https://github.com/buxtronix/phev2mqtt/pull/62)
to close leaked connections and release blocked message queues. It also fixes
a reader/pinger data race and limits each TCP connection attempt to ten seconds.
See [reconnect notes](docs/reconnecting.md) for attribution, behavior, and tests.

For climate command troubleshooting, see [climate diagnostics](docs/climate-diagnostics.md).

## Charging status

Version 1.7 adds a Home Assistant **Charge State** sensor on
`<prefix>/charge/state`. It reports `charging`, `waiting`, `unplugged`, `idle`,
or `error`. Waiting means the car reports waiting for its charging timer;
error means it reports a charge-timer error. Idle means connected but not
charging, without a waiting/error flag; it does not imply a full battery.

Unknown data and reconnects make the sensor unavailable until fresh status
arrives. The existing `<prefix>/charge/charging` topic and Charging binary
sensor still publish only `on`/`off`, so existing automations keep working.
See [docs/charge-state.md](docs/charge-state.md) for decoding and precedence.

## Charge timer override (experimental)

Version 1.6 adds an experimental Home Assistant switch for the charge timer
override. It is based on PHEV Link protocol evidence. Resume-timer / override OFF
was acknowledged and verified by status read-back on the user’s MY20. See
[docs/charge-override.md](docs/charge-override.md) for provenance and limitations.

| Topic | Direction | Behavior |
|---|---|---|
| `<prefix>/set/charge/override` | command | `on` writes register `0x17` with `01`; `off` writes register `0x17` with `02` |
| `<prefix>/set/cancelchargetimer` | command | Legacy alias for the same validated `on`/`off` behavior |
| `<prefix>/charge/override` | state | `on`, `off`, or `unknown`, derived only from incoming register `0x1b` notifications |
| `<prefix>/register/1b` | state | Existing raw register `0x1b` hex value |

Payloads other than `on` or `off` (case-insensitive with surrounding whitespace
trimmed) are rejected without writing anything. The switch stays unavailable until
a fresh valid register `0x1b` notification arrives; commands do not set optimistic
state. Unknown data and reconnections invalidate the previous state. The client
now resynchronizes its send key when the car requests it; the 10-second write
ACK timeout is preserved. See the protocol notes for validation and limitations.

## Development workspace

For the mounted sandbox checkouts, retained research, artifacts, and publication
state, see [the workspace map](docs/workspace.md).
