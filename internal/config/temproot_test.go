package config_test

import (
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/osroot"
)

// tempRepoDir is t.TempDir plus a release of every process-wide osroot handle
// that a test rooted there can leave behind.
//
// osroot.Shared caches an open *os.Root per directory for the lifetime of the
// process and deliberately never closes it. On Windows an open directory
// handle blocks removal, so t.TempDir's own cleanup fails with "The process
// cannot access the file because it is being used by another process" for
// every test that opened one. Cleanups run last-in-first-out and TempDir
// registers its own before this one, so the handles are released first.
//
// Three things decide the anchor list:
//
// Both spellings of the path are covered, because osroot keys its registry on
// the exact string it was handed. Production code reaches these directories
// through paths.WorktreeRoot, which resolves the canonical path, while
// t.TempDir returns whatever the platform calls its temp root: Windows hands
// out an 8.3 short path (C:\Users\RUNNER~1\...) where git reports the long
// one, and macOS hands out /var/... for /private/var/....
//
// The parent is covered too. StateStore and LocalManifestStore anchor on
// filepath.Dir of the directory they are given and address the leaf by name —
// so a test that passes its t.TempDir straight to NewStateStoreWithDir opens a
// root on the temp *base*, one level above the directory under test.
//
// Only one level up, never further: the next level is the shared system temp
// directory, and closing a root on that would pull it out from under every
// other test in the process.
//
// Enumerated rather than reset wholesale, because osroot.ResetShared would
// close roots that concurrently running parallel tests are still using. Forget
// is documented as idempotent for a directory that was never opened, so naming
// a candidate this test never created costs nothing.
func tempRepoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() {
		bases := []string{dir}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
			bases = append(bases, resolved)
		}
		for _, base := range bases {
			for _, p := range []string{
				filepath.Join(base, ".git", "entire-investigations", "manifests"),
				filepath.Join(base, ".git", "entire-investigations"),
				filepath.Join(base, ".entire"),
				filepath.Join(base, ".git"),
				base,
				filepath.Dir(base),
			} {
				osroot.Forget(p)
			}
		}
	})
	return dir
}
