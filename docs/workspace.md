# Persistent sandbox workspace

The only retained mount is `/workspace/phev`. The main add-on checkout and its
Git history live directly there. Everything else worth keeping is under
`.local/`, ignored by Git and excluded from Docker's build context.

| Location, relative to the mounted folder | Contents |
|---|---|
| `phev2mqtt/` | Tracked, vendored bridge source used by the add-on Dockerfile |
| `.local/repos/phev2mqtt/` | Standalone bridge Git checkout; origin is `gnyman/phev2mqtt`, branch `master` |
| `.local/repos/ha-addons/` | App-store Git checkout; origin is `gnyman/ha-addons`, branch `main` |
| `.local/repos/ha-addons/phev/` | Pinned add-on submodule for the store entry |
| `.local/research/phev/` | Official/PHEV Link APKs, decoded smali/Java, public source references, jadx tools |
| `.local/reviews/pr62/` | Isolated PR #62 review worktree, including original diagnostic tests |
| `.local/history/` | Preserved scratch snapshots, original research logs, captures, session exports, and build/test outputs |
| `.local/artifacts/phev-addon-1.8.tar.gz` | Deployable source archive of the add-on |

The `.local/` directory is intentionally hidden: Supervisor recursively scans
non-hidden directories for app metadata, and these checkouts contain reference
configurations that must not appear as additional apps. It is also excluded
from the Docker context so APKs, private captures, and repository metadata are
not uploaded to the builder or accidentally published.

## Continue development

```sh
cd /workspace/phev/.local/repos/phev2mqtt
go test -race ./... -count=1 -timeout=45s
go vet ./...
```

The PR #62 review worktree is attached to this standalone bridge repository.
Git's worktree pointers have been repaired for the mounted locations. The
app-store's `phev` submodule uses relative Git metadata and keeps its public
GitHub source URL.

Edits to the standalone bridge do not automatically change the tracked bridge
subtree. Commit the bridge first, fetch that commit into the main add-on, and
merge it with `git subtree merge --prefix=phev2mqtt <bridge-commit>`. Follow the
[subtree workflow](development.md) for publishing updates.

## Current publication state

PHEV 1.8 is prepared for `gnyman/ha-addons`, but nothing has been pushed. Publish
in this order once GitHub authentication is available:

1. `.local/repos/phev2mqtt` → `gnyman/phev2mqtt` (`master`).
2. The main checkout → `gnyman/home-assistant-addon-phev2mqtt` (`main`).
3. `.local/repos/ha-addons` → `gnyman/ha-addons` (`main`).

The store entry must point to a published add-on commit. Authentication is not
saved in the retained session exports. Keep credentials in the normal GitHub
connection/credential flow rather than source files or archives.

The latest live vehicle action restored timer control: `0x17=02` was acknowledged
and `0x1b=02` read back. No further vehicle probing is needed to publish or test
the reconnect lifecycle fixes. Historical captures and helper programs in
`.local/history/` are records, not startup/deployment scripts.

## Tooling and history

OpenCode history is backed up under `.local/history/opencode/`: portable session
exports, a consistent SQLite database snapshot, logs, and saved tool outputs.
The original provider authentication file is not copied. Use `opencode import`
on a session JSON to restore a session into a newly installed OpenCode.

Public research tools are preserved in `.local/research/phev/tools/jadx/`.
Java, apktool, Go, libpcap development headers, and other installed system
packages belong to the disposable container and may need reinstalling. Prefer
`uv` for Python and `bun` for JavaScript, as requested for this sandbox.
