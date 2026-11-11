# Reconnect cleanup

This fork incorporates [upstream PR #62](https://github.com/buxtronix/phev2mqtt/pull/62),
original commit `45675c3a8d53d250557ef155671c37de456980a6` by
willhonor <will@cohesis.co.uk>. Its imported commit preserves the original
authorship and co-author credit, with an explicit upstream PR reference.

The upstream fix closes connections after failed handshakes and other attempt
exits. It makes message queuing and receive delivery abort on client closure,
closes listener channels, and stops commands from waiting on a closed client.
This prevents an abandoned connection from continuing to ping the car and
occupying its single client slot. Our existing XOR-key resynchronization and
charging-state reporting are retained.

Follow-up changes in this fork:

- Synchronize the last-receive timestamp shared by reader and pinger. A local
  traffic test exposed this existing data race after applying the upstream PR.
- Bound TCP dialing to ten seconds, and cancel an in-progress dial on Close.
  Discard and close a connection that arrives after the client has closed.
- Register the manager listener before starting the reader, so an immediate
  disconnect cannot strand a late listener. A listener added after Close is
  already closed.
- Keep repeated handshake notifications from blocking manager shutdown; Start
  also stops waiting when the client closes.
- Close clients on exits from the watch, set, and registration commands too.

Offline regression tests use loopback sockets or net.Pipe, never a vehicle.
They cover the real 20-second handshake timeout, connection release, full
send/receive queues, listener closure, repeated handshakes, traffic under the
race detector, bounded dialing, cancellation, and a late successful dial.

```sh
go test -race ./... -count=1 -timeout=45s
go vet ./...
go build .
```

A dial timeout still means the endpoint could not be reached in time. This
patch does not restore Wi-Fi, forwarding, or a powered-down vehicle. Repeated
encoding complaints remain visible and still trigger the existing disconnect
threshold; the change ensures that a failed attempt is cleaned up before the
next one. Live car validation was not performed for these lifecycle changes.
