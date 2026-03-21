package cmd

import (
	"fmt"
	"os"

	"github.com/babelcloud/dy2text/internal/downloader"
	"github.com/babelcloud/dy2text/internal/resolver"
	"github.com/babelcloud/dy2text/internal/transcriber"
	"github.com/spf13/cobra"
)

var (
	apiKey   string
	endpoint string
	model    string
	language string
	prompt   string
	keepAudio bool
	outputFile string
)

var rootCmd = &cobra.Command{
	Use:   "dy2text <douyin-url>",
	Short: "Transcribe Douyin video speech to text",
	Long: `dy2text extracts speech from Douyin (抖音) videos and converts it to text.

It takes a Douyin video link as input, downloads the audio track using yt-dlp,
and transcribes it using the OpenAI Whisper API.

Prerequisites:
  - yt-dlp: brew install yt-dlp
  - ffmpeg: brew install ffmpeg
  - OpenAI API key: export OPENAI_API_KEY=sk-...`,
	Args: cobra.ExactArgs(1),
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key (or set OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&endpoint, "endpoint", "", "Whisper API endpoint (for compatible APIs)")
	rootCmd.Flags().StringVar(&model, "model", "whisper-1", "Whisper model to use")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "Language hint (ISO 639-1, e.g., zh, en)")
	rootCmd.Flags().StringVar(&prompt, "prompt", "", "Context prompt to guide transcription")
	rootCmd.Flags().BoolVar(&keepAudio, "keep-audio", false, "Keep the downloaded audio file")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write transcript to file instead of stdout")
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	inputURL := args[0]

	// Check dependencies
	if err := downloader.CheckDependencies(); err != nil {
		return err
	}

	// Resolve URL
	fmt.Fprintf(os.Stderr, "Resolving URL...\n")
	videoURL, err := resolver.Resolve(inputURL)
	if err != nil {
		return fmt.Errorf("resolving URL: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Resolved: %s\n", videoURL)

	// Download audio
	fmt.Fprintf(os.Stderr, "Downloading audio...\n")
	audioPath, err := downloader.DownloadAudio(videoURL, "")
	if err != nil {
		return fmt.Errorf("downloading audio: %w", err)
	}
	if !keepAudio {
		defer downloader.Cleanup(audioPath)
	} else {
		fmt.Fprintf(os.Stderr, "Audio saved: %s\n", audioPath)
	}
	fmt.Fprintf(os.Stderr, "Audio downloaded: %s\n", audioPath)

	// Transcribe
	fmt.Fprintf(os.Stderr, "Transcribing...\n")
	opts := transcriber.Options{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		Language: language,
		Prompt:   prompt,
	}
	text, err := transcriber.Transcribe(audioPath, opts)
	if err != nil {
		return fmt.Errorf("transcribing: %w", err)
	}

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(text+"\n"), 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Transcript saved to: %s\n", outputFile)
	} else {
		fmt.Println(text)
	}

	return nil
}
