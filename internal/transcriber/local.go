package transcriber

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const defaultLocalModel = "base"

// LocalOptions configures local whisper.cpp transcription.
type LocalOptions struct {
	// ModelPath is the path to a ggml model file. If empty, auto-downloads to ~/.dyt/models/.
	ModelPath string
	// ModelName selects which model to download (tiny, base, small, medium, large-v3).
	// Empty means the caller did not choose one, so an already installed model is used first.
	ModelName string
	// Language is an optional ISO 639-1 language code hint.
	Language string
	// Threads is the number of threads. 0 = auto.
	Threads int
}

// TranscribeLocal uses whisper.cpp CLI to transcribe audio locally (no API key needed).
func TranscribeLocal(audioPath string, opts LocalOptions) (string, error) {
	whisperBin, err := findWhisperBin()
	if err != nil {
		return "", err
	}

	// Convert audio to 16kHz WAV (whisper.cpp requirement)
	wavPath, err := convertToWav(audioPath)
	if err != nil {
		return "", fmt.Errorf("converting to WAV: %w", err)
	}
	defer os.Remove(wavPath)

	modelPath := opts.ModelPath
	if modelPath == "" && !modelNameSet(opts) {
		if found := findInstalledModel(); found != "" {
			modelPath = found
			fmt.Fprintf(os.Stderr, "Using installed model: %s\n", modelPath)
		}
	}
	if modelPath == "" {
		name := opts.ModelName
		if name == "" {
			name = defaultLocalModel
		}
		modelPath, err = ensureModel(name)
		if err != nil {
			return "", fmt.Errorf("preparing model: %w", err)
		}
	}

	args := []string{
		"-m", modelPath,
		"-f", wavPath,
		"--no-timestamps", "-np",
	}
	if opts.Language != "" {
		args = append(args, "-l", opts.Language)
	}
	if opts.Threads > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Threads))
	}

	cmd := exec.Command(whisperBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper-cpp failed: %w\nOutput: %s", err, string(out))
	}

	text := strings.TrimSpace(string(out))
	// whisper.cpp may output some metadata lines; extract just the transcription
	text = cleanWhisperOutput(text)
	return text, nil
}

// findWhisperBin finds the whisper-cpp binary.
func findWhisperBin() (string, error) {
	// Try common names
	for _, name := range []string{"whisper-cli", "whisper-cpp", "whisper", "main"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	// macOS brew location
	if runtime.GOOS == "darwin" {
		brewPath := "/opt/homebrew/bin/whisper-cli"
		if _, err := os.Stat(brewPath); err == nil {
			return brewPath, nil
		}
	}
	return "", fmt.Errorf("whisper-cpp not found: install with 'brew install whisper-cpp' or download from https://github.com/ggerganov/whisper.cpp")
}

// convertToWav converts audio to 16kHz mono WAV using ffmpeg.
func convertToWav(audioPath string) (string, error) {
	dir := filepath.Dir(audioPath)
	wavPath := filepath.Join(dir, "dyt-whisper-input.wav")

	cmd := exec.Command("ffmpeg", "-y", "-i", audioPath,
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w\nOutput: %s", err, string(out))
	}
	return wavPath, nil
}

// modelsDir returns ~/.dyt/models/ and ensures it exists.
func modelsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".dyt", "models")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// modelNameSet reports whether the caller asked for a specific model name.
func modelNameSet(opts LocalOptions) bool {
	return opts.ModelName != ""
}

// preferredModels is the search order when the caller did not name a model.
// Larger models hear names better, so an installed one wins over downloading base.
var preferredModels = []string{
	"ggml-large-v3-turbo.bin",
	"ggml-large-v3.bin",
	"ggml-large-v2.bin",
	"ggml-medium.bin",
	"ggml-small.bin",
	"ggml-base.bin",
	"ggml-tiny.bin",
}

// findInstalledModel looks in the directories whisper models usually live in.
func findInstalledModel() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dirs := []string{
		filepath.Join(home, ".local", "share", "whisper-cpp"),
		filepath.Join(home, ".dyt", "models"),
		filepath.Join(home, ".cache", "whisper"),
	}
	for _, name := range preferredModels {
		for _, dir := range dirs {
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err == nil && info.Size() > 1024*1024 {
				return path
			}
		}
	}
	return ""
}

// ensureModel downloads the model if not already present.
func ensureModel(name string) (string, error) {
	dir, err := modelsDir()
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("ggml-%s.bin", name)
	modelPath := filepath.Join(dir, filename)

	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	// Download from huggingface
	url := fmt.Sprintf("https://huggingface.co/ggerganov/whisper.cpp/resolve/main/%s", filename)
	fmt.Fprintf(os.Stderr, "Downloading model %s...\n", name)
	fmt.Fprintf(os.Stderr, "URL: %s\n", url)
	fmt.Fprintf(os.Stderr, "Destination: %s\n", modelPath)

	cmd := exec.Command("curl", "-L", "--progress-bar", "-o", modelPath, url)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(modelPath) // Clean up partial download
		return "", fmt.Errorf("downloading model: %w", err)
	}

	// Verify file is not empty/error page
	info, err := os.Stat(modelPath)
	if err != nil || info.Size() < 1024*1024 {
		os.Remove(modelPath)
		return "", fmt.Errorf("downloaded model appears invalid (too small); check the model name: %s", name)
	}

	fmt.Fprintf(os.Stderr, "Model downloaded: %s (%.0f MB)\n", modelPath, float64(info.Size())/(1024*1024))
	return modelPath, nil
}

// cleanWhisperOutput strips metadata/header lines from whisper.cpp output.
func cleanWhisperOutput(raw string) string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip whisper.cpp info/debug lines
		if strings.HasPrefix(line, "whisper_") ||
			strings.HasPrefix(line, "main:") ||
			strings.HasPrefix(line, "ggml_") ||
			strings.HasPrefix(line, "system_info:") ||
			strings.HasPrefix(line, "load_backend:") ||
			strings.HasPrefix(line, "read_audio_data:") {
			continue
		}
		// Remove special tokens like [_EOT_], [_BEG_], etc.
		line = regexp.MustCompile(`\[_[A-Z]+_\]`).ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// CheckLocalDependencies checks if whisper.cpp and ffmpeg are available.
func CheckLocalDependencies() error {
	if _, err := findWhisperBin(); err != nil {
		return err
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: install with 'brew install ffmpeg'")
	}
	return nil
}
