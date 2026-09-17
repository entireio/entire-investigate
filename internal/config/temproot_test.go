package config_test

import (
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/osroot"
)

// tempRepoDir is t.TempDir plus a release of every process-wide osroot handle
// that a test repository rooted there can leave behind.
//
// osroot.Shared caches an open *os.Root per directory for the lifetime of the
// process and deliberately never closes it. On Windows an open directory
// handle blocks removal, so t.TempDir's own cleanup fails with "The process
// cannot access the file because it is being used by another process" for
// every test that opened one. Cleanups run last-in-first-out and TempDir
// registers its own before this one, so the handles are released first.
//
// The anchors are enumerated rather than reset wholesale: osroot.ResetShared
// would close roots that concurrently running parallel tests are still using.
// Forget is documented as idempotent for a directory that was never opened, so
// naming a candidate that this test never created costs nothing.
func tempRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() {
		for _, p := range []string{
			filepath.Join(dir, ".git", "entire-investigations", "manifests"),
			filepath.Join(dir, ".git", "entire-investigations"),
			filepath.Join(dir, ".entire"),
			filepath.Join(dir, ".git"),
			dir,
		} {
			osroot.Forget(p)
		}
	})
	return dir
}
