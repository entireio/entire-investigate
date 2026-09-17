// Package bridge supplies the investigate command with the handful of
// behaviours that lived in the Entire CLI's own cli package before the command
// moved out. Each is reimplemented here against exported CLI packages, so the
// plugin never imports the monolithic cli package.
package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/agent/claudecode"
	"github.com/entireio/cli/cmd/entire/cli/agent/codex"
	"github.com/entireio/cli/cmd/entire/cli/agent/geminicli"
	"github.com/entireio/cli/cmd/entire/cli/agent/spawn"
	agenttypes "github.com/entireio/cli/cmd/entire/cli/agent/types"
	"github.com/entireio/cli/cmd/entire/cli/checkpoint"
	"github.com/entireio/cli/cmd/entire/cli/gitrepo"
	"github.com/entireio/cli/cmd/entire/cli/logging"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/strategy"
	"github.com/entireio/cli/cmd/entire/cli/trailers"
)

// AgentsWithHooksInstalled mirrors cli.GetAgentsWithHooksInstalled.
func AgentsWithHooksInstalled(ctx context.Context) []agenttypes.AgentName {
	var installed []agenttypes.AgentName
	for _, name := range agent.List() {
		ag, err := agent.Get(name)
		if err != nil {
			continue
		}
		hs, ok := agent.AsHookSupport(ag)
		if !ok {
			continue
		}
		if ok, err := hs.AreHooksInstalled(ctx); ok {
			installed = append(installed, name)
		} else if err != nil && ctx.Err() == nil {
			logging.Debug(ctx, "hooks-installed check failed", slog.String("agent", string(name)))
		}
	}
	return installed
}

// SpawnerFor mirrors cli.launchableSpawnerFor.
func SpawnerFor(agentName string) spawn.Spawner {
	switch agentName {
	case string(agent.AgentNameClaudeCode):
		return claudecode.NewSpawner()
	case string(agent.AgentNameCodex):
		return codex.NewSpawner()
	case string(agent.AgentNameGemini):
		return geminicli.NewSpawner()
	default:
		return nil
	}
}

// HeadHasInvestigateCheckpoint mirrors cli.headHasInvestigateCheckpoint.
func HeadHasInvestigateCheckpoint(ctx context.Context) (bool, string) {
	repoRoot, err := paths.WorktreeRoot(ctx)
	if err != nil {
		return false, ""
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "log", "-1", "--format=%B").Output()
	if err != nil {
		return false, ""
	}
	cpID, ok := trailers.ParseCheckpoint(string(out))
	if !ok {
		return false, ""
	}
	repo, err := gitrepo.OpenPath(repoRoot)
	if err != nil {
		return false, ""
	}
	defer repo.Close()
	stores, err := checkpoint.Open(ctx, repo, checkpoint.OpenOptions{ReadRemotes: strategy.CheckpointReadRemotes(ctx)})
	if err != nil {
		return false, ""
	}
	summary, err := checkpoint.ReadCheckpoint(ctx, stores.Persistent, cpID)
	if err != nil || summary == nil || !summary.HasInvestigation {
		return false, ""
	}
	return true, fmt.Sprintf("checkpoint %s", cpID)
}
