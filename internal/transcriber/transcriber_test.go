package transcriber

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTranscribe_NoAPIKey(t *testing.T) {
	// Ensure env var is not set for this test
	orig := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("OPENAI_API_KEY", orig)
		}
	}()

	_, err := Transcribe("dummy.mp3", Options{})
	if err == nil {
		t.Error("Transcribe should error when no API key is set")
	}
}

func TestTranscribe_FileNotFound(t *testing.T) {
	_, err := Transcribe("/nonexistent/file.mp3", Options{APIKey: "sk-test"})
	if err == nil {
		t.Error("Transcribe should error when file doesn't exist")
	}
}

func TestTranscribe_MockServer(t *testing.T) {
	// Create a mock Whisper API server
	expectedText := "你好，这是一个测试视频"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-test-key" {
			t.Errorf("expected Bearer sk-test-key, got %s", auth)
		}

		// Parse multipart
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("parsing form: %v", err)
		}

		// Check model field
		model := r.FormValue("model")
		if model != "whisper-1" {
			t.Errorf("expected model whisper-1, got %s", model)
		}

		// Check language field
		lang := r.FormValue("language")
		if lang != "zh" {
			t.Errorf("expected language zh, got %s", lang)
		}

		// Check file was uploaded
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("getting form file: %v", err)
		}
		defer file.Close()
		if header.Filename != "test.mp3" {
			t.Errorf("expected filename test.mp3, got %s", header.Filename)
		}

		// Return response
		resp := Response{Text: expectedText}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Create a temp audio file
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.mp3")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Transcribe(audioPath, Options{
		APIKey:   "sk-test-key",
		Endpoint: ts.URL,
		Language: "zh",
	})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if got != expectedText {
		t.Errorf("Transcribe = %q, want %q", got, expectedText)
	}
}

func TestTranscribe_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Invalid API key"}}`)
	}))
	defer ts.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake"), 0644)

	_, err := Transcribe(audioPath, Options{
		APIKey:   "sk-invalid",
		Endpoint: ts.URL,
	})
	if err == nil {
		t.Error("Transcribe should error on server error")
	}
}

func TestTranscribe_WithPrompt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)

		prompt := r.FormValue("prompt")
		if prompt != "这是一个科技视频" {
			t.Errorf("expected prompt '这是一个科技视频', got %s", prompt)
		}

		resp := Response{Text: "test"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake"), 0644)

	_, err := Transcribe(audioPath, Options{
		APIKey:   "sk-test",
		Endpoint: ts.URL,
		Prompt:   "这是一个科技视频",
	})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
}

func TestTranscribe_CustomModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)

		model := r.FormValue("model")
		if model != "whisper-large-v3" {
			t.Errorf("expected model whisper-large-v3, got %s", model)
		}

		resp := Response{Text: "test"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake"), 0644)

	_, err := Transcribe(audioPath, Options{
		APIKey:   "sk-test",
		Endpoint: ts.URL,
		Model:    "whisper-large-v3",
	})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
}

func TestTranscribe_APIKeyFromEnv(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-from-env" {
			t.Errorf("expected Bearer sk-from-env, got %s", auth)
		}

		resp := Response{Text: "test"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake"), 0644)

	orig := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "sk-from-env")
	defer func() {
		if orig != "" {
			os.Setenv("OPENAI_API_KEY", orig)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	_, err := Transcribe(audioPath, Options{
		Endpoint: ts.URL,
	})
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
}
