# entire-investigate

🔎 Multi-agent investigation plugin for the [Entire CLI](https://github.com/entireio/cli).

Agents take turns appending findings, evidence, and analysis to a shared findings document until they reach quorum. When they're done, `fix` hands the accepted findings to a coding agent as grounded context.

This ships as an Entire [external command](https://github.com/entireio/cli/blob/main/docs/architecture/external-commands.md): install the binary and `entire investigate` dispatches to it. It was previously built into the CLI.

## Install

```sh
entire plugin install investigate
```

Or from source:

```sh
mise run install
```

## Use

```sh
entire investigate                       # investigate the current branch
entire investigate notes.md              # seed from a findings doc
entire investigate --issue-link <url>    # seed from a GitHub issue or PR
entire investigate --continue <run-id>   # resume an interrupted run
entire investigate --findings            # browse saved investigations
entire investigate show <run-id>         # print a saved investigation
entire investigate fix <run-id>          # launch a coding agent with the findings
entire investigate clean <run-id>|--all  # delete saved artifacts
entire investigate --edit                # re-open the config picker
```

Run `entire investigate --help` for the full flag list.

## Configuration

First run offers a picker and writes `.entire/investigate.local.json`:

```json
{
  "agents": ["claude-code", "codex"],
  "max_turns": 2,
  "quorum": 0,
  "always_prompt": ""
}
```

| Field | Meaning |
| --- | --- |
| `agents` | Ordered list of agents to round-robin during the loop |
| `max_turns` | Per-agent turn budget (default 2) |
| `quorum` | Approvals needed to terminate; `0` means all agents must approve |
| `always_prompt` | Text appended to every turn's composed prompt |

### The config file is deliberately per-clone

There is no committed counterpart, and that is a security property rather than an oversight.

`always_prompt` is free text that lands verbatim in the prompt of an agent this plugin spawns with approval checks disabled (`bypassPermissions` for claude-code, `--dangerously-bypass-approvals-and-sandbox` for codex). If it could be read from a version-controlled file, an ordinary pull request would steer a permission-bypassed agent on every developer who pulled it. With no committed file in the lookup order there is nothing to gate.

Two things back that up rather than leaving it to convention:

- **The file is ignored on first write.** `Save` adds `investigate.local.json` to `.entire/.gitignore`, appending so the CLI's own entries survive. A stray `git add -A` would otherwise track it and silently downgrade the config.
- **A tracked file loses the prompt.** If it is committed anyway, the plugin's index check drops `always_prompt` and reports why; `agents`, `max_turns` and `quorum` still apply.

`.entire` itself is opened as a name inside a root anchored on the worktree root, never by resolving the joined path, so a repository-controlled `.entire` symlink is refused instead of followed. Without that, a committed symlink would put the real config somewhere the index records under a different name — the untracked check would pass and version-controlled text would reach an approvals-disabled agent.

This mirrors the CLI's own treatment of `.entire/settings.local.json`; see `internal/config` for the reasoning in full.

### Migrating from the built-in

`entire investigate` previously read an `investigate` block inside `.entire/settings.json` / `.entire/settings.local.json`. That block is no longer read — copy the inner object into `.entire/investigate.local.json`, or just run `entire investigate --edit` and pick again. The field names are unchanged.

The Entire CLI still accepts a leftover `investigate` key in its settings files so nothing breaks if you leave it there, but it ignores it.

## Artifacts

Runs are stored outside `.entire/`, in the git common directory, so they never show up as working-tree changes:

```
<git-common-dir>/entire-investigations/
  manifests/            # run manifests, for --findings
  <run-id>/
    findings.md
    state.json
```

`entire investigate clean` removes them.

## Development

```sh
mise run build      # build the binary
mise run test       # unit tests
mise run lint       # vet, golangci-lint, gofmt, go mod tidy, shellcheck
mise run check      # everything, plus cross-builds
mise run install    # build and install into the managed plugin dir
```

The plugin depends on `github.com/entireio/cli` for shared infrastructure — agent registry and spawners, session and checkpoint state, settings, git and filesystem helpers — rather than vendoring copies of it. It needs CLI `v0.10.7` or newer.

## Licence

MIT. See [LICENSE](LICENSE).
