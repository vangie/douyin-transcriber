// Package transcriber handles speech-to-text using OpenAI Whisper API.
package transcriber

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultWhisperEndpoint = "https://api.openai.com/v1/audio/transcriptions"
	defaultModel           = "whisper-1"
)

// Options configures the transcription.
type Options struct {
	// APIKey is the OpenAI API key. If empty, reads from OPENAI_API_KEY env var.
	APIKey string
	// Endpoint overrides the Whisper API endpoint (for compatible APIs).
	Endpoint string
	// Model is the Whisper model to use. Default: "whisper-1".
	Model string
	// Language is an optional ISO 639-1 language code hint (e.g., "zh", "en").
	Language string
	// Prompt is optional context to guide transcription.
	Prompt string
}

// Response is the transcription result.
type Response struct {
	Text string `json:"text"`
}

// Transcribe sends an audio file to the Whisper API and returns the transcript.
func Transcribe(audioPath string, opts Options) (string, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not set: use --api-key flag or OPENAI_API_KEY environment variable")
	}

	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = defaultWhisperEndpoint
	} else {
		endpoint = transcriptionURL(endpoint)
	}

	model := opts.Model
	if model == "" {
		model = defaultModel
	}

	// Open audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("opening audio file: %w", err)
	}
	defer file.Close()

	// Build multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("copying file data: %w", err)
	}

	if err := writer.WriteField("model", model); err != nil {
		return "", fmt.Errorf("writing model field: %w", err)
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("writing response_format field: %w", err)
	}
	if opts.Language != "" {
		if err := writer.WriteField("language", opts.Language); err != nil {
			return "", fmt.Errorf("writing language field: %w", err)
		}
	}
	if opts.Prompt != "" {
		if err := writer.WriteField("prompt", opts.Prompt); err != nil {
			return "", fmt.Errorf("writing prompt field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing multipart writer: %w", err)
	}

	// Send request
	req, err := http.NewRequest("POST", endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Whisper API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	return result.Text, nil
}

// Describe is the endpoint and model printed while a cloud transcription runs.
func Describe(base, model string) string {
	if base == "" {
		base = defaultWhisperEndpoint
	} else {
		base = transcriptionURL(base)
	}
	if model == "" {
		model = defaultModel
	}
	return base + " model " + model
}

// transcriptionURL accepts either the full transcriptions URL or an OpenAI
// compatible base such as https://host/v1.
func transcriptionURL(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/audio/transcriptions") {
		return endpoint
	}
	return endpoint + "/audio/transcriptions"
}
