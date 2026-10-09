package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The installer must not wipe the whole data\ folder: the thumbnail cache
// lives in data\thumbnail beside the executable.
func TestInstallerKeepsThumbnailCache(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "installer", "livepaper.nsi"))
	if err != nil {
		t.Skipf("installer script not available: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(l, ";") {
			continue
		}
		if strings.HasPrefix(l, "rmdir") && strings.Contains(l, `\data"`) && strings.Contains(l, "/r") {
			t.Errorf("installer recursively removes the data folder, which holds the thumbnail cache: %s", line)
		}
	}
}
