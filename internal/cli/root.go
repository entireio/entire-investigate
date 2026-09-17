package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/entireio/entire-investigate/internal/agentlaunch"
	"github.com/entireio/entire-investigate/internal/bridge"
	"github.com/entireio/entire-investigate/internal/investigate"
)

// Options configures the plugin root command.
type Options struct {
	Version string
	Env     EntireEnv
}

// Execute runs the plugin with the real process environment.
//
// Signals are handled here rather than left to the default disposition: the
// investigate loop spawns agent subprocesses, and a Ctrl-C that killed this
// process outright would orphan them and lose the run state that makes
// `--continue` work. NotifyContext cancels the context instead, which the loop
// already treats as "stop after the current turn and persist".
func Execute(version string) error {
	err := run(version)

	// A silent error has already been reported with its own message; main
	// must not print it a second time. Exiting happens here, after run's
	// deferred signal-reset has already executed.
	var silent silentError
	if errors.As(err, &silent) {
		os.Exit(1)
	}
	return err
}

// run owns the signal context so its deferred stop is not skipped by the
// os.Exit in Execute.
func run(version string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := NewRootCommand(Options{Version: version, Env: EnvFromOS()})
	if err := cmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("entire-investigate: %w", err)
	}
	return nil
}

// NewRootCommand builds the `entire-investigate` command tree.
//
// The root IS the investigate command rather than a wrapper around it, so
// `entire investigate --findings` and `entire investigate show <id>` keep
// working exactly as they did when this was a built-in subcommand. `version`
// and `doctor` are the plugin-contract additions every Entire plugin carries.
func NewRootCommand(opts Options) *cobra.Command {
	if opts.Version == "" {
		opts.Version = "dev"
	}

	cmd := investigate.NewCommand(investigate.Deps{
		GetAgentsWithHooksInstalled:  bridge.AgentsWithHooksInstalled,
		NewSilentError:               func(err error) error { return silentError{err} },
		SpawnerFor:                   bridge.SpawnerFor,
		LaunchFix:                    agentlaunch.LaunchFixAgent,
		HeadHasInvestigateCheckpoint: bridge.HeadHasInvestigateCheckpoint,
	})

	cmd.Use = "entire-investigate [seed-doc]"
	// The parent CLI prints returned errors itself when dispatching an
	// external command, and cobra's usage dump on a runtime failure buries
	// the actual message.
	cmd.SilenceErrors = true
	// Not hidden here: as a plugin the command is opt-in by installation, so
	// the labs-style concealment it carried as a built-in no longer applies.
	cmd.Hidden = false

	cmd.AddCommand(newDoctorCommand(opts.Env))
	cmd.AddCommand(newVersionCommand(opts.Version))
	return cmd
}

// silentError marks an error whose message the command already printed.
// It mirrors the parent CLI's NewSilentError contract.
type silentError struct{ err error }

func (e silentError) Error() string { return e.err.Error() }
func (e silentError) Unwrap() error { return e.err }
