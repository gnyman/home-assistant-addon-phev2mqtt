# Repository layout

The add-on repository is https://github.com/gnyman/home-assistant-addon-phev2mqtt
(`origin`, branch `main`). Bridge source is vendored as a full-history Git
subtree in `phev2mqtt/`, from https://github.com/gnyman/phev2mqtt
(`phev2mqtt` remote, branch `master`). Normal clones include the source; no
submodule initialization or build-time Git clone is needed.

A separate bridge checkout can be used to commit and test standalone bridge
patches. After bridge commits are available from its remote, update the add-on
with `git subtree pull --prefix=phev2mqtt phev2mqtt master`. To extract changes
made directly to the vendored source, use
`git subtree split --prefix=phev2mqtt -b bridge-export`, then review them before
publishing to the bridge fork. Do not change the source revision implicitly in
the Dockerfile.

The bridge fork was fast-forwarded from c698b3a to buxtronix/phev2mqtt commit
8f423405eef2f833a6a90a93a577624c5875a24c before applying our patches, preserving
upstream authorship. Existing upstream Go module/import paths are retained.

AI implementation authors use the names `gpt-6.1` and `deepseek-flash` with
placeholder emails under `ai.invalid`; Gabriel N <gabriel@nyman.re> is credited
with a Co-authored-by trailer on each session-created commit.
