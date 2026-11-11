# Home Assistant add-on: PHEV to MQTT

Source: https://github.com/gnyman/home-assistant-addon-phev2mqtt

Bridge: https://github.com/gnyman/phev2mqtt (vendored in `phev2mqtt/`).

This requires that Home Assistant is already connected to the Outlander, you should be able to check this by ensuring that you can ping 192.168.8.46

Once installed as a local addon, you must configure the mqtt target and username/password before start and stop phev2mqtt from the GUI
For my docker install the host machine's address (which is also running the mqtt addon) is 172.30.32.1:1883 (I suspect this could be different depending on install)

These files need to go in a directory (called phev for example) in the local addons.

If you ssh into the main os, this will be in /usr/share/hassio/addons/local/phev
If you ssh in using the terminal add on, this will be /addons/phev (or symblink /root/addons/phev)
If you connect to the hassio_supervisort docker container, this will be /data/addons/local/phev

More information about local addons at https://developers.home-assistant.io/docs/add-ons/tutorial/

If you set the debug flag on, it will just start the container and sleep indefinitely, you can then run the phev2mqtt manually, to run "phev2mqtt client watch" for example.

## Reconnecting

Version 1.8 incorporates [upstream PR #62](https://github.com/buxtronix/phev2mqtt/pull/62)
to close leaked connections and release blocked message queues. It also fixes
a reader/pinger data race and limits each TCP connection attempt to ten seconds.
See [reconnect notes](docs/reconnecting.md) for attribution, behavior, and tests.

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

NOTE - on 32bit Raspberry Pi there's an issue with the latest alpine docker images and old versions of libseccomp (including the ones in Raspbian repos), this will stop the Dockerfile building. In order to overcome this, you may need to manually install a newer version with

    wget http://ftp.us.debian.org/debian/pool/main/libs/libseccomp/libseccomp2_2.4.4-1~bpo10+1_armhf.deb
    dpkg -i libseccomp2_2.4.4-1~bpo10+1_armhf.deb
