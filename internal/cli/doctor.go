package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/entireio/entire-investigate/internal/config"
)

func newDoctorCommand(env EntireEnv) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the parent Entire CLI plugin environment and this plugin's config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(cmd, env)
		},
	}
}

// runDoctor reports the plugin contract environment and the config file's
// state. Unlike the template's doctor it does not require
// ENTIRE_PLUGIN_DATA_DIR: investigate keeps its run artifacts in the git
// common dir (<git-common-dir>/entire-investigations/), not in per-user plugin
// storage, so an unset data dir is not a fault to report.
func runDoctor(cmd *cobra.Command, env EntireEnv) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "%s=%s\n", envCLIVersion, valueOrUnset(env.CLIVersion))
	fmt.Fprintf(out, "%s=%s\n", envRepoRoot, valueOrUnset(env.RepoRoot))
	fmt.Fprintf(out, "%s=%s\n", envPluginDataDir, valueOrUnset(env.PluginDataDir))

	path, err := config.Path(ctx)
	if err != nil {
		fmt.Fprintf(out, "config: not resolvable (%v)\n", err)
		return nil
	}
	fmt.Fprintf(out, "config: %s\n", path)

	res, err := config.Load(ctx)
	if err != nil {
		fmt.Fprintf(out, "config: unreadable (%v)\n", err)
		return nil
	}
	switch {
	case res.Config.IsZero():
		fmt.Fprintln(out, "config: unset — `entire investigate` will run first-time setup")
	default:
		fmt.Fprintf(out, "config: agents=%v max_turns=%d quorum=%d\n",
			res.Config.Agents, res.Config.MaxTurns, res.Config.Quorum)
	}
	if res.PromptRejection != "" {
		fmt.Fprintf(out, "config: always_prompt dropped — %s\n", res.PromptRejection)
	}
	return nil
}
