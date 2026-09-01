package version

import "testing"

func TestDefaults(t *testing.T) {
	t.Parallel()
	if Version == "" || Commit == "" {
		t.Fatal("empty version")
	}
}
