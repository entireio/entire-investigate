// Package config owns `entire investigate`'s configuration.
//
// The config lives in .entire/investigate.local.json, a per-clone file that is
// never committed. It used to live under the "investigate" key of the Entire
// CLI's own .entire/settings.json, but that file is decoded with
// DisallowUnknownFields, so every plugin-owned key had to be a field the CLI
// itself shipped — which is the opposite of what an external command is for.
// Owning a separate file removes that coupling entirely: the CLI never parses
// this one.
//
// There is deliberately no committed counterpart. AlwaysPrompt is free text
// that lands verbatim in the prompt of an agent spawned with approval checks
// disabled, so a version-controlled source for it would let an ordinary pull
// request steer a permission-bypassed agent on everyone who pulls. With no
// committed file in the lookup order there is nothing to gate — the class of
// attack is designed out rather than defended against. VerifyUntracked covers
// the one case that survives: someone committing this file anyway.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/entireio/cli/cmd/entire/cli/gitrepo"
	"github.com/entireio/cli/cmd/entire/cli/jsonutil"
	"github.com/entireio/cli/cmd/entire/cli/osroot"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

// FileName is the config file's name inside .entire/.
const FileName = "investigate.local.json"

// RepoRelPath is the config file's path relative to the worktree root, in the
// slash-separated form git stores index entries under, on every platform.
const RepoRelPath = ".entire/" + FileName

// Config is the persisted `entire investigate` configuration. The field names
// match the schema this used to have under the CLI's "investigate" settings
// key, so a user moving an old block across keeps the same spellings.
type Config struct {
	// Agents is the ordered list of agent names to round-robin during the loop.
	Agents []string `json:"agents,omitempty"`

	// MaxTurns is the per-agent turn budget. Zero means the package default.
	MaxTurns int `json:"max_turns,omitempty"`

	// Quorum is the count of `approve` stances needed to terminate the loop.
	// Zero means "all agents must approve".
	Quorum int `json:"quorum,omitempty"`

	// AlwaysPrompt is appended to every turn's composed prompt.
	//
	// Honored only from this untracked file. Load drops it (and reports why)
	// when the file turns out to be tracked, because a committed one arrives
	// by cloning rather than from the developer running the command.
	AlwaysPrompt string `json:"always_prompt,omitempty"`
}

// IsZero reports whether the config is effectively unset.
func (c *Config) IsZero() bool {
	if c == nil {
		return true
	}
	return len(c.Agents) == 0 && c.MaxTurns == 0 && c.Quorum == 0 && c.AlwaysPrompt == ""
}

// Result carries the loaded config plus any downgrade applied to it.
type Result struct {
	Config *Config

	// PromptRejection explains why AlwaysPrompt was dropped, or "" when it was
	// applied (or never set). Consumers print it, because an instruction the
	// user can see in a file but that is not in effect is otherwise invisible.
	PromptRejection string
}

// Load reads the config. A missing file is not an error: it returns a zero
// Config so first-run setup can offer the picker.
//
// Decoding is lenient. This file is read by exactly one program, but it is
// read by every version of that program that has ever shipped, and refusing a
// key an older plugin does not understand would break it permanently for that
// user with no way for us to fix it for them.
func Load(ctx context.Context) (*Result, error) {
	dir, err := entireDir(ctx)
	if errors.Is(err, fs.ErrNotExist) {
		// No .entire/ at all — the same "not configured yet" state as a
		// missing file, not a fault. A repo that has never run `entire
		// enable` must still reach first-run setup.
		return &Result{Config: &Config{}}, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := osroot.ReadFileNoFollow(dir, FileName)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Result{Config: &Config{}}, nil
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", RepoRelPath, err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", RepoRelPath, err)
	}

	res := &Result{Config: cfg}
	if cfg.AlwaysPrompt != "" {
		if reason := VerifyUntracked(ctx); reason != "" {
			cfg.AlwaysPrompt = ""
			res.PromptRejection = reason
		}
	}
	return res, nil
}

// Save writes cfg to .entire/investigate.local.json, creating .entire/ if it
// does not exist. The write is atomic and replaces a leaf symlink rather than
// following it.
func Save(ctx context.Context, cfg *Config) error {
	root, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return fmt.Errorf("locate worktree root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".entire"), 0o750); err != nil {
		return fmt.Errorf("create .entire: %w", err)
	}
	dir, err := entireDir(ctx)
	if err != nil {
		return err
	}
	data, err := jsonutil.MarshalIndentWithNewline(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", RepoRelPath, err)
	}
	if err := jsonutil.WriteFileAtomicIn(dir, FileName, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", RepoRelPath, err)
	}
	return nil
}

// Path returns the absolute path of the config file.
func Path(ctx context.Context) (string, error) {
	root, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return "", fmt.Errorf("locate worktree root: %w", err)
	}
	return filepath.Join(root, ".entire", FileName), nil
}

// VerifyUntracked returns "" when the config file is genuinely this
// developer's own, or a human-readable reason when it is not and AlwaysPrompt
// must therefore be dropped.
//
// Only the git index is consulted, not HEAD. The index is what a delivered
// attack shows up in: a pull request that commits the file puts it in the
// index of every clone that checks the branch out. The state the index misses
// — committed, then `git rm --cached` — is one the local developer created,
// not one that arrived with the repository. This mirrors the CLI's own
// classifyLocalSettings, whose shallow form is the documented right cost for
// the broad population; the deep index-and-HEAD form is reserved there for
// fields that pick a binary to execute.
//
// Fails closed: an unreadable repository drops the prompt, because being wrong
// means an attacker steering a permission-bypassed agent rather than a user
// losing a preference.
func VerifyUntracked(ctx context.Context) string {
	root, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return "the worktree root could not be resolved"
	}
	repo, err := gitrepo.OpenPath(root)
	if err != nil {
		// No repository at all: the file cannot have arrived by cloning when
		// there is nothing to clone from.
		if errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		return "the repository could not be read to verify the file is untracked"
	}
	defer func() { _ = repo.Close() }()

	idx, err := repo.Storer.Index()
	if err != nil {
		return "the git index could not be read to verify the file is untracked"
	}
	for _, entry := range idx.Entries {
		if entry.Name == RepoRelPath {
			return RepoRelPath + " is tracked by git, so it may have arrived by cloning"
		}
	}
	return ""
}

// entireDir returns the shared root anchored at <worktree>/.entire. The root is
// owned by osroot's process-wide registry and must not be closed.
func entireDir(ctx context.Context) (*os.Root, error) {
	root, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("locate worktree root: %w", err)
	}
	dir, err := osroot.Shared(filepath.Join(root, ".entire"))
	if err != nil {
		// osroot.Shared returns a missing directory unwrapped so callers can
		// classify it; preserve that for Load's errors.Is check.
		if errors.Is(err, fs.ErrNotExist) {
			//nolint:wrapcheck // Load classifies this with errors.Is; wrapping is safe for errors.Is but the bare sentinel keeps the contract obvious at both ends.
			return nil, err
		}
		return nil, fmt.Errorf("open .entire: %w", err)
	}
	return dir, nil
}
