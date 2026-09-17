package investigate

import (
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/osroot"
)

// tempRepoDir is t.TempDir plus a release of any process-wide osroot handle on
// the directory's .entire subdirectory.
//
// osroot.Shared caches an open *os.Root per directory for the lifetime of the
// process and deliberately never closes it. On Windows an open directory
// handle blocks removal, so t.TempDir's own cleanup fails with "The process
// cannot access the file because it is being used by another process" for
// every test that caused .entire to be opened. Cleanups run last-in-first-out
// and TempDir registers its own before this one, so the handle is released
// first.
//
// Forget is idempotent for a directory that was never opened, so this is safe
// for temp dirs that hold no .entire at all.
func tempRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() { osroot.Forget(filepath.Join(dir, ".entire")) })
	return dir
}
