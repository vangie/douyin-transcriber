package cmd

import (
	"fmt"
	"os"

	"github.com/babelcloud/douyin-transcriber/internal/downloader"
	"github.com/babelcloud/douyin-transcriber/internal/resolver"
	"github.com/babelcloud/douyin-transcriber/internal/transcriber"
	"github.com/spf13/cobra"
)

var (
	apiKey     string
	endpoint   string
	model      string
	language   string
	prompt     string
	keepAudio  bool
	outputFile string
	local      bool
	modelPath  string
	modelName  string
	threads    int
)

var rootCmd = &cobra.Command{
	Use:   "dyt <douyin-url>",
	Short: "Transcribe Douyin video speech to text",
	Long: `dyt extracts speech from Douyin (抖音) videos and converts it to text.

It takes a Douyin video link as input, downloads the audio track using yt-dlp,
and transcribes it using OpenAI Whisper API or local whisper.cpp.

Modes:
  Cloud (default): Uses OpenAI Whisper API (requires OPENAI_API_KEY)
  Local (--local): Uses whisper.cpp (no API key needed, runs on your machine)

Prerequisites:
  - yt-dlp: brew install yt-dlp
  - ffmpeg: brew install ffmpeg
  - Cloud mode: export OPENAI_API_KEY=sk-...
  - Local mode: brew install whisper-cpp`,
	Args: cobra.ExactArgs(1),
	RunE: run,
}

func init() {
	// Cloud mode flags
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key (or set OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&endpoint, "endpoint", "", "Whisper API endpoint (for compatible APIs)")

	// Local mode flags
	rootCmd.Flags().BoolVar(&local, "local", false, "Use local whisper.cpp instead of OpenAI API")
	rootCmd.Flags().StringVar(&modelPath, "model-path", "", "Path to local ggml model file")
	rootCmd.Flags().StringVar(&modelName, "model-name", "base", "Local model to download (tiny, base, small, medium, large-v3)")

	// Shared flags
	rootCmd.Flags().StringVar(&model, "model", "whisper-1", "Whisper model (cloud mode)")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "Language hint (ISO 639-1, e.g., zh, en)")
	rootCmd.Flags().StringVar(&prompt, "prompt", "", "Context prompt to guide transcription (cloud mode)")
	rootCmd.Flags().BoolVar(&keepAudio, "keep-audio", false, "Keep the downloaded audio file")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write transcript to file instead of stdout")
	rootCmd.Flags().IntVarP(&threads, "threads", "t", 0, "Number of threads for local mode (0 = auto)")
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
	if local {
		if err := transcriber.CheckLocalDependencies(); err != nil {
			return err
		}
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
	var text string
	if local {
		fmt.Fprintf(os.Stderr, "Mode: local (whisper.cpp)\n")
		text, err = transcriber.TranscribeLocal(audioPath, transcriber.LocalOptions{
			ModelPath: modelPath,
			ModelName: modelName,
			Language:  language,
			Threads:   threads,
		})
	} else {
		fmt.Fprintf(os.Stderr, "Mode: cloud (OpenAI Whisper API)\n")
		text, err = transcriber.Transcribe(audioPath, transcriber.Options{
			APIKey:   apiKey,
			Endpoint: endpoint,
			Model:    model,
			Language: language,
			Prompt:   prompt,
		})
	}
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
