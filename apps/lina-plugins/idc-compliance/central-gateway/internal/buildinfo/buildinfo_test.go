package buildinfo

import "testing"

func TestCurrentNormalizesReleaseMetadata(t *testing.T) {
	originalVersion, originalCommit, originalBuiltAt := version, commit, builtAt
	t.Cleanup(func() {
		version, commit, builtAt = originalVersion, originalCommit, originalBuiltAt
	})
	version = " v1.2.3 "
	commit = " "
	builtAt = "2026-08-13T00:00:00Z"

	got := Current()
	if got.Service != serviceName || got.Version != "v1.2.3" || got.Commit != "unknown" ||
		got.BuiltAt != "2026-08-13T00:00:00Z" {
		t.Fatalf("unexpected build info: %+v", got)
	}
}
