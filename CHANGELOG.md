# Version 1.10.2
* Build and advertise only the supported Home Assistant architectures: aarch64 and amd64
* Disable unsupported i386, armhf and armv7 image targets, and remove obsolete 32-bit installation guidance

# Version 1.10.1
* Make debug enable protocol debug logging while the bridge keeps running, instead of pausing the app
* Hide routine ping requests and responses by default; enable log_pings to include them
* Print the app version and source commit on startup for build identification
* Keep register writes, acknowledgements, encoding complaints and key resynchronization visible; report encoding retry counts on write timeouts
* Exercise climate wire payload, ACK matching and encoding recovery against in-memory simulated peers without controlling a real vehicle

# Version 1.10
* Publish tested images for all five supported architectures to GHCR; Home Assistant downloads the image instead of compiling on the device
* Use a stripped static bridge and options launcher in a minimal runtime with TLS certificates, without an OS, shell, Bashio, or unused libpcap packages
* Keep configuration keys and MQTT discovery identities; debug mode pauses the bridge and handles shutdown signals
* Disable the standalone bridge WiFi restart shell command because Home Assistant manages the host network

# Version 1.9.1
* Set a default Home Assistant base image so builds work when Supervisor does not provide BUILD_FROM (2026.04 and later)

# Version 1.9
* Rename the app to PHEV-DRAGONS with the distinct `phev-dragons` slug so the old local PHEV installation can be kept

# Version 1.8
* Remove obsolete nested add-on metadata so Home Assistant discovers exactly one PHEV app
* Incorporate [upstream PR #62](https://github.com/buxtronix/phev2mqtt/pull/62), preserving its original authorship, to release leaked connections and unblock shutdown/reconnection
* Add a ten-second cancellable TCP dial deadline and synchronize the reader/pinger timestamp
* Cover failed handshakes, full queues, listener shutdown, repeated handshakes, and dial cancellation with offline race-enabled tests

# Version 1.7
* Add a Home Assistant Charge State sensor on `<prefix>/charge/state`: charging, waiting, unplugged, idle, or error
* Decode timer waiting/error from register `0x1e`, giving these flags priority over the charging bit
* Keep `/charge/charging` and its binary sensor unchanged for existing automations
* Invalidate the new sensor on reconnect or unknown data; require fresh charging data after plug-state changes

# Version 1.6
* Recover the transmit-key position from the car’s encoding complaints; verify resume-timer on MY20 by write ACK and `0x1b=02` notification
* Invalidate override availability on unknown data and car/MQTT reconnections
* Vendor the phev2mqtt build source and build it from the local add-on copy instead of cloning from git
* Fix `init` to be a JSON boolean (`false`) and bump the add-on version to 1.6
* Add an experimental charge timer override switch: ON writes register `0x17` with `01`, OFF writes register `0x17` with `02`
* Add `<prefix>/set/charge/override`; keep `<prefix>/set/cancelchargetimer` as a validated legacy alias for `on`/`off`
* Publish `<prefix>/charge/override` only from incoming register `0x1b` notifications; status stays unavailable until fresh valid data arrives
* Replace the old incorrect switch discovery (charging state / legacy command topic) while retaining the existing unique id and discovery topic
* Add offline unit tests and charge override documentation with third-party provenance

# Version 1.5
* Run with init: false to to allow it to start on newer HA instances. See https://developers.home-assistant.io/blog/2022/05/12/s6-overlay-base-images/
* Accept phev_ip and phev_port as parameters in case your PHEV is elsewhere

# Version 1.4
* Changed "startup": "before" to "startup": "application" as "before" no longer supported and it should really start after Home Assistant

# Version 1.3
* Removed host networking as it wasn't required

# Version 1.2
* Removed fatal error if Outlander is not available

# Version 1.1
* Set inital values of mqtt_user and mqtt_password to null instead of dummy values to ensure that user enters them
* Added debug variable to allow start of container and then manual connection to it to run commands
* Added netcat test to check that Outlander is available
* Moved phev2mqtt binary from /tmp/phev2mqtt to /opt

# Version 1
Initial build
