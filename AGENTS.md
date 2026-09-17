# AGENTS.md — operating guide for coding agents

`entire-investigate` is an [Entire CLI](https://github.com/entireio/cli) external command. Built as `entire-investigate` on `$PATH`, the parent CLI dispatches it when a user runs `entire investigate`. `CLAUDE.md` inherits this file.

The command was a built-in in `entireio/cli` until it moved here. Nothing about its behaviour changed in the move, so its history in that repo is still the best explanation of why any given piece works the way it does.

## Project map

- `cmd/entire-investigate/`: entry point. Thin — it calls `internal/cli`.
- `internal/cli/`: the plugin shell — root command, `version`, `doctor`, signal handling.
- `internal/investigate/`: the command itself. Loop, TUI, bootstrap, findings, fix, show, clean, picker, manifests, run state.
- `internal/investigate/flowchart/`: the run diagram `show` renders.
- `internal/config/`: this plugin's configuration file and the trust check on it.
- `internal/bridge/`: the few behaviours that used to come from the CLI's own `cli` package.
- `internal/agentlaunch/`: launching a normal coding agent with a composed prompt, for `fix`.

## It depends on the Entire CLI as a module

`github.com/entireio/cli` is an ordinary dependency, and `internal/investigate` imports its packages directly — the agent registry and spawners, session and checkpoint state, settings, git and filesystem helpers. That is deliberate: those carry contracts (session kinds, checkpoint metadata, `ENTIRE_INVESTIGATE_*` env names) that the CLI reads back, and a vendored copy would drift out of agreement with the binary actually running the hooks.

Two consequences worth knowing before you change anything here:

- **Never import `github.com/entireio/cli/cmd/entire/cli` itself.** It pulls in the whole CLI. Anything needed from it gets reimplemented in `internal/bridge` against exported packages; that package exists for exactly this reason.
- **The CLI's repo conventions apply to code that came from it.** `.golangci.yaml` mirrors the CLI's so the moved sources lint identically. When touching filesystem, git or settings code, read the matching reference in the CLI repo's `docs/development/` rather than inventing a local rule.

## Verification

| Task | Command |
| --- | --- |
| Build | `mise run build` |
| Unit tests | `mise run test` |
| Tests with race detection | `mise run test:ci` |
| Format | `mise run fmt` |
| Lint (vet, golangci-lint, gofmt, go mod tidy, shellcheck) | `mise run lint` |
| Cross-build every plugin target | `mise run build-all` |
| Everything, before a commit | `mise run check` |
| Build and install into the managed plugin dir | `mise run install` |

CI runs unit tests on **Linux, macOS and Windows**. The CLI repo runs them on Linux only, so code moved from there has never been exercised on the other two — see the filesystem note below for the class of failure that produces.

## Things that bite

**Config lives in `.entire/investigate.local.json`, never in the CLI's settings.** The CLI decodes `.entire/settings.json` with `DisallowUnknownFields`, so a plugin-owned key there makes the *whole CLI* fail to load settings in that repo. `internal/config` owns this file end to end; the CLI never parses it.

**`always_prompt` is an instruction channel into an approvals-disabled agent.** It is honored only from this untracked file, and `config.VerifyUntracked` drops it when the file is in the git index. There is deliberately no committed counterpart — with nothing committed in the lookup order there is no layer to gate. Do not add one.

**Open `.entire` as a name inside a root anchored on the worktree root.** Never resolve `<worktree>/.entire` and open that: `os.OpenRoot` resolves every component first, so a repository-controlled `.entire` symlink is followed before confinement begins, and the untracked check above can then be satisfied by a file the index records under another name. `entireDir` uses `MkdirAllNoSymlink` + `SharedChild`, which is the CLI's own pattern (`entiredir.openDir`). Regression tests cover both the read and the write path.

**`osroot.Shared` never closes its roots, and Windows will not delete an open directory.** Any test that causes one to be opened fails `t.TempDir` cleanup on Windows. Use `tempRepoDir` rather than `t.TempDir` in tests; it forgets the anchors afterwards, covering both path spellings (Windows 8.3 short paths, macOS `/var` vs `/private/var`) and the store's parent directory. Do not reach for `osroot.ResetShared` — it closes roots that parallel tests are still using.

**Golden files are compared byte-for-byte against `\n`-built strings.** `.gitattributes` pins LF; without it every prompt golden fails on a Windows checkout with a diff that prints identically on both sides.

**Run artifacts live outside `.entire`,** under `<git-common-dir>/entire-investigations/` — `manifests/` plus `<run-id>/{findings.md,state.json}`. They are per-clone and must never be committed or walked into a checkpoint tree.

## Releasing

A `v*` tag triggers `.github/workflows/release.yml`, which runs goreleaser and publishes the archives plus `checksums.txt` that `entire plugin install` verifies. Until a release exists, the only way to install is a local build (`mise run install`) — the bare name resolves through the plugin index and the URL form resolves a version with `git ls-remote --tags`, so both need a published tag.
