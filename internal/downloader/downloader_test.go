package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDependencies(t *testing.T) {
	err := CheckDependencies()
	if err != nil {
		t.Skipf("Dependencies not installed: %v", err)
	}
}

func TestExtractVideoIDFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.douyin.com/video/7616660340146752819", "7616660340146752819"},
		{"https://www.douyin.com/video/123/", "123"},
	}
	for _, tt := range tests {
		got, err := extractVideoIDFromURL(tt.url)
		if err != nil {
			t.Fatalf("extractVideoIDFromURL(%q) error: %v", tt.url, err)
		}
		if got != tt.want {
			t.Errorf("extractVideoIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestCleanup(t *testing.T) {
	dir, err := os.MkdirTemp("", "dyt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	audioPath := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(audioPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	Cleanup(audioPath)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Cleanup should have removed %s", dir)
		os.RemoveAll(dir)
	}
}

func TestCleanup_NonTempDir(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(audioPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	Cleanup(audioPath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Cleanup should not have removed non-temp dir")
	}
}
