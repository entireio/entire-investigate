package investigate

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
// Both spellings of the path are forgotten. osroot keys its registry on the
// exact string it was handed, and the production code reaches these
// directories through paths.WorktreeRoot — which resolves the *canonical*
// path, while t.TempDir returns whatever the platform's temp root is called.
// The two differ on both CI platforms that matter: Windows hands out an 8.3
// short path (C:\Users\RUNNER~1\...) where git reports the long one, and macOS
// hands out /var/... where the real path is /private/var/.... Forgetting only
// the t.TempDir spelling misses the cached key entirely.
//
// The anchors are enumerated rather than reset wholesale: osroot.ResetShared
// would close roots that concurrently running parallel tests are still using.
// Forget is documented as idempotent for a directory that was never opened, so
// naming a candidate this test never created costs nothing.
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
			} {
				osroot.Forget(p)
			}
		}
	})
	return dir
}
