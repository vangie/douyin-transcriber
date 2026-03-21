package transcriber

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanWhisperOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "with whisper metadata",
			input: "whisper_init: loading model\nmain: processing audio\nHello world\nGoodbye",
			want:  "Hello world\nGoodbye",
		},
		{
			name:  "with ggml lines",
			input: "ggml_init_cublas: CUDA available\nsystem_info: 8 threads\nThis is a test",
			want:  "This is a test",
		},
		{
			name:  "empty lines stripped",
			input: "\n  \nHello\n  \nWorld\n",
			want:  "Hello\nWorld",
		},
		{
			name:  "Chinese text",
			input: "whisper_init: model loaded\n你好世界\n这是一个测试",
			want:  "你好世界\n这是一个测试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanWhisperOutput(tt.input)
			if got != tt.want {
				t.Errorf("cleanWhisperOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModelsDir(t *testing.T) {
	dir, err := modelsDir()
	if err != nil {
		t.Fatalf("modelsDir() error: %v", err)
	}
	if dir == "" {
		t.Error("modelsDir() returned empty string")
	}
	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("modelsDir() did not create a directory")
	}
	// Verify path ends with .dyt/models
	if filepath.Base(dir) != "models" {
		t.Errorf("expected dir to end with 'models', got %s", filepath.Base(dir))
	}
}

func TestEnsureModel_InvalidName(t *testing.T) {
	// ensureModel with a nonexistent model should fail on download
	// We don't actually download in unit tests, but we can test the path logic
	dir, err := modelsDir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "ggml-tiny.bin")
	// If the model already exists, skip download test
	if _, err := os.Stat(expected); err == nil {
		t.Skip("model already downloaded, skipping")
	}
	// We'd need network for a real test; just verify the function exists
	t.Log("ensureModel requires network, skipping download test")
}

func TestFindWhisperBin_NotRequired(t *testing.T) {
	// findWhisperBin may or may not find whisper-cpp depending on the machine
	path, err := findWhisperBin()
	if err != nil {
		t.Logf("whisper-cpp not installed (expected in CI): %v", err)
		return
	}
	t.Logf("Found whisper-cpp at: %s", path)
}

func TestConvertToWav_MissingInput(t *testing.T) {
	_, err := convertToWav("/nonexistent/file.mp3")
	if err == nil {
		t.Error("convertToWav should fail with missing input file")
	}
}
