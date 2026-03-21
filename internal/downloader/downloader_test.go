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

func TestFindAudioFile(t *testing.T) {
	// Create temp dir with audio file
	dir := t.TempDir()

	// No audio file
	_, err := findAudioFile(dir)
	if err == nil {
		t.Error("findAudioFile should error when no audio file exists")
	}

	// Create a fake mp3 file
	mp3Path := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(mp3Path, []byte("fake mp3 data"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := findAudioFile(dir)
	if err != nil {
		t.Fatalf("findAudioFile error: %v", err)
	}
	if got != mp3Path {
		t.Errorf("findAudioFile = %q, want %q", got, mp3Path)
	}
}

func TestFindAudioFile_MultipleFormats(t *testing.T) {
	formats := []string{".mp3", ".m4a", ".wav", ".ogg", ".opus", ".flac"}

	for _, ext := range formats {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "audio"+ext)
			if err := os.WriteFile(path, []byte("fake audio"), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := findAudioFile(dir)
			if err != nil {
				t.Fatalf("findAudioFile error for %s: %v", ext, err)
			}
			if got != path {
				t.Errorf("findAudioFile = %q, want %q", got, path)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	// Create a temp dir that looks like our naming convention
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
		os.RemoveAll(dir) // clean up anyway
	}
}

func TestCleanup_NonTempDir(t *testing.T) {
	// Should not delete directories that don't match our naming convention
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
