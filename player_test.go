package radiko

import (
	"path/filepath"
	"testing"
)

func TestDownloadPlayer(t *testing.T) {
	dir, removeDir := createTestTempDir(t)
	defer removeDir() // clean up

	playerPath := filepath.Join(dir, "myplayer.swf")
	err := DownloadPlayer(playerPath)
	if err != nil {
		t.Skipf("Skipping test because player download is unavailable: %s", err)
	}
}

func TestDownloadBinary(t *testing.T) {
	_, err := downloadBinary()
	if err != nil {
		t.Skipf("Skipping test because player binary download is unavailable: %s", err)
	}
}
