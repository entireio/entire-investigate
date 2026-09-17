package investigate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/entireio/cli/cmd/entire/cli/testutil"

	"github.com/entireio/entire-investigate/internal/config"
)

// writeConfig writes an always_prompt-bearing config into repo's .entire/.
func writeConfig(t *testing.T, repo string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".entire"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".entire", config.FileName),
		[]byte(`{"agents":["claude-code"],"always_prompt":"be skeptical"}`), 0o644))
}

// Not parallel: uses t.Chdir().
//
// always_prompt lands verbatim in the prompt of an agent spawned with approval
// checks disabled. A config file that is tracked by git arrived by cloning
// rather than from the developer running the command, so the prompt must be
// dropped — and the run path must say so, because a preamble that silently
// stops applying is indistinguishable from one nobody wrote.
func TestConfigLoad_TrackedFileDropsAlwaysPrompt(t *testing.T) {
	tmp := tempRepoDir(t)
	testutil.InitRepo(t, tmp)
	writeConfig(t, tmp)
	testutil.GitAdd(t, tmp, filepath.Join(".entire", config.FileName))
	t.Chdir(tmp)

	res, err := config.Load(context.Background())
	require.NoError(t, err)
	require.Empty(t, res.Config.AlwaysPrompt, "a tracked config file must not supply always_prompt")
	require.NotEmpty(t, res.PromptRejection, "the drop must be reported")
	require.Equal(t, []string{"claude-code"}, res.Config.Agents,
		"only always_prompt is gated; inert fields still apply")

	var buf bytes.Buffer
	notifyDroppedInvestigatePrompt(&buf, res)
	out := buf.String()
	assert.Contains(t, out, "always_prompt", "the notice names the dropped field")
	assert.Contains(t, out, config.RepoRelPath, "the notice names the file to untrack")
}

// Not parallel: uses t.Chdir().
//
// The ordinary case: an untracked config file is this developer's own, so
// always_prompt applies and nothing is reported.
func TestConfigLoad_UntrackedFileKeepsAlwaysPrompt(t *testing.T) {
	tmp := tempRepoDir(t)
	testutil.InitRepo(t, tmp)
	writeConfig(t, tmp)
	t.Chdir(tmp)

	res, err := config.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "be skeptical", res.Config.AlwaysPrompt)
	require.Empty(t, res.PromptRejection)

	var buf bytes.Buffer
	notifyDroppedInvestigatePrompt(&buf, res)
	assert.Empty(t, buf.String(), "nothing to report when the prompt applies")
}
