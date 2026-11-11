# Version 1.6
* Vendor the phev2mqtt build source and build it from the local add-on copy instead of cloning from git
* Fix `init` to be a JSON boolean (`false`) and bump the add-on version to 1.6
* Add an experimental charge timer override switch: ON writes register `0x17` with `01`, OFF writes register `0x17` with `02`
* Add `<prefix>/set/charge/override`; keep `<prefix>/set/cancelchargetimer` as a validated legacy alias for `on`/`off`
* Publish `<prefix>/charge/override` only from incoming register `0x1b` notifications; initial state stays unknown until one arrives
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
