# Small prebuilt images

The source repository workflow `.github/workflows/publish.yaml` tests the Go
bridge and options launcher, cross-compiles static binaries, builds and smoke-tests
both supported platforms, then publishes versioned images:

`ghcr.io/gnyman/{arch}-addon-phev-dragons:1.10.2`

Architectures: aarch64 (ARM64) and amd64 (x86-64). Home Assistant no longer
supports the 32-bit i386, armhf, and armv7 architectures, so their image builds
are disabled. Cross-compilation runs on the native build host; QEMU is used
only for runtime smoke tests.
Each workflow job reports unpacked image bytes and compressed registry layer bytes.

## First publication

1. Push the source branch `phev-dragons` to
   `gnyman/home-assistant-addon-phev2mqtt`. Pushes to `main` also run the workflow.
2. Wait for every image job to pass. Pull-request builds test without publishing.
3. In the source repository's GitHub Packages settings, make both new
   container packages **public**. A public repository does not guarantee a new
   package is public. Confirm each image can be pulled without authentication.
4. Only then push the `ha-addons` commit pinning this source revision. Its
   `image` property switches Supervisor from local compilation to registry pulls.

The source workflow needs `packages: write` (declared in the workflow), and the
repository's Actions settings must allow its GITHUB_TOKEN to publish packages.
The ha-addons legacy builder skips this app; the source workflow owns publishing.
Never replace a published version tag with different source: bump the version.

## Size

Locally measured ARM64 payload for 1.10: approximately **4.4 MB compressed** and
**11.1 MB of unpacked files**, including the stripped bridge, options launcher,
Mozilla TLS certificate bundle, and license. This is an estimate of the shipped
files, not a measured registry image; Docker layer compression and the builder's
certificate bundle may change it slightly. Workflow summaries provide the actual
published sizes.

For comparison, the previous `ghcr.io/home-assistant/base:latest` ARM64 base alone
was **17.8 MB compressed** at inspection time, before the bridge and libpcap-dev.
The Go compiler was already excluded by the old multi-stage Dockerfile.

## Runtime differences

Configuration keys, MQTT topics and discovery identities are preserved. The
launcher reports the app version and source commit, reads `/data/options.json`,
and replaces itself with the bridge so the bridge receives Supervisor shutdown
signals directly. Optional null MQTT
credentials become empty strings. Both numeric and string port values are
accepted. TLS root certificates remain available.

There is no shell, Bashio, package manager, or packet-capture support in the
image. The pcap command was already excluded from ordinary builds by its build
tag. For diagnostics, execute `/opt/phev2mqtt` directly. Debug mode starts the
bridge with `--verbosity debug`. Routine ping traffic is hidden unless
`log_pings: true` is set. Host WiFi is managed by HA; the
standalone Linux WiFi restart shell command is explicitly disabled.

Go modules and compiled packages are cached during image builds. Only stripped
static binaries and runtime files are copied to the final scratch image.
