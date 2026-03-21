// Package downloader handles audio extraction from Douyin videos using yt-dlp.
package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckDependencies verifies that yt-dlp and ffmpeg are installed.
func CheckDependencies() error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found: install it with 'brew install yt-dlp' or 'pip install yt-dlp'")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: install it with 'brew install ffmpeg'")
	}
	return nil
}

// DownloadAudio downloads the audio track from a Douyin video URL.
// It returns the path to the downloaded audio file.
// If outputDir is empty, a temporary directory is used.
func DownloadAudio(videoURL, outputDir string) (string, error) {
	if outputDir == "" {
		dir, err := os.MkdirTemp("", "dy2text-*")
		if err != nil {
			return "", fmt.Errorf("creating temp dir: %w", err)
		}
		outputDir = dir
	}

	outputTemplate := filepath.Join(outputDir, "audio.%(ext)s")

	args := []string{
		"--no-warnings",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--output", outputTemplate,
		"--no-playlist",
		"--user-agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
		videoURL,
	}

	cmd := exec.Command("yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w\noutput: %s", err, string(output))
	}

	// Find the downloaded audio file
	audioPath, err := findAudioFile(outputDir)
	if err != nil {
		return "", fmt.Errorf("finding audio file: %w\nyt-dlp output: %s", err, string(output))
	}

	return audioPath, nil
}

// findAudioFile locates the audio file in the given directory.
func findAudioFile(dir string) (string, error) {
	audioExtensions := []string{".mp3", ".m4a", ".wav", ".ogg", ".opus", ".flac"}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		for _, audioExt := range audioExtensions {
			if ext == audioExt {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("no audio file found in %s", dir)
}

// Cleanup removes the temporary directory created during download.
func Cleanup(path string) {
	if strings.Contains(path, "dy2text-") {
		os.RemoveAll(filepath.Dir(path))
	}
}
