package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vangie/douyin-transcriber/internal/cache"
	"github.com/vangie/douyin-transcriber/internal/downloader"
	"github.com/vangie/douyin-transcriber/internal/models"
	"github.com/vangie/douyin-transcriber/internal/polish"
	"github.com/vangie/douyin-transcriber/internal/resolver"
	"github.com/vangie/douyin-transcriber/internal/transcriber"
)

var (
	apiKey     string
	baseURL    string
	endpoint   string
	speech     string
	speechURL  string
	chat       string
	language   string
	prompt     string
	keepAudio  bool
	outputFile string
	noPolish   bool
	rawOutput  string
	listOnly   bool
	history    bool
	forget     string
)

var rootCmd = &cobra.Command{
	Use:   "dyt <douyin-url>",
	Short: "Transcribe Douyin video speech to text",
	Long: `dyt extracts speech from Douyin (抖音) videos and converts it to text.

It takes a Douyin video link (short or full URL) as input, downloads the audio
track directly from Douyin's CDN, and transcribes it using OpenAI Whisper API
or local whisper.cpp.

A speech model and a chat model are chosen separately. They do not share a name.

  dyt URL                                      best local ggml file, then the best chat model
  dyt --speech small URL                       that ggml model; chat model stays on priority
  dyt --chat llama3.1:8b URL                   that chat model; speech model stays on priority
  dyt --speech small --chat llama3.1:8b URL    both named
  dyt --speech-url https://host/v1 URL         remote /v1/audio/transcriptions, model whisper-1
  dyt --speech whisper-1 --speech-url https://host/v1 URL
  dyt --no-polish URL                          transcribe only
  dyt --list                                   print the models that were found

A local speech model is a ggml file. The larger file wins.
A remote speech model is --speech-url plus the name in --speech. It is not a chat model.
Chat models come from Magpie, then Ollama, then --base-url.
--chat ollama-first tries Ollama before Magpie.

Prerequisites:
  - ffmpeg: brew install ffmpeg
  - a local ggml whisper model`,
	Args: cobra.MaximumNArgs(1),
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVar(&speech, "speech", "", "Speech model. A local ggml name or path, or the model name on --speech-url")
	rootCmd.Flags().StringVar(&speechURL, "speech-url", "", "Remote speech service, an OpenAI-compatible /v1 that accepts /audio/transcriptions")
	rootCmd.Flags().StringVar(&chat, "chat", "", "Chat model for rewriting: a name from --list, such as llama3.1:8b")
	rootCmd.Flags().StringVar(&baseURL, "base-url", "", "Also look for chat models at this OpenAI-compatible /v1 base")
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "Bearer token for --speech-url and --base-url (or OPENAI_API_KEY)")
	rootCmd.Flags().BoolVar(&noPolish, "no-polish", false, "Transcribe only")
	rootCmd.Flags().BoolVar(&listOnly, "list", false, "List discovered models and exit")
	rootCmd.Flags().BoolVar(&history, "history", false, "List transcripts cached in /tmp/dyt and exit")
	rootCmd.Flags().StringVar(&forget, "forget", "", "Delete one cached video by the id shown in --history")
	rootCmd.Flags().StringVarP(&language, "language", "l", "", "Language hint for the speech model (zh, en)")
	rootCmd.Flags().StringVar(&prompt, "prompt", "", "Hint passed to a remote speech model")
	rootCmd.Flags().BoolVar(&keepAudio, "keep-audio", false, "Keep the downloaded audio file")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write the transcript to a file")
	rootCmd.Flags().StringVar(&rawOutput, "raw-output", "", "Also write the transcript before it is rewritten")
	rootCmd.Flags().StringVar(&endpoint, "endpoint", "", "Deprecated. Use --base-url")
	_ = rootCmd.Flags().MarkHidden("endpoint")
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	if endpoint != "" && baseURL == "" {
		baseURL = endpoint
	}
	key := apiKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	found := models.Discover(models.DiscoverOptions{BaseURL: baseURL, APIKey: key})
	if listOnly {
		printModels(found)
		return nil
	}
	if history {
		return printHistory()
	}
	if forget != "" {
		if err := cache.Remove(forget); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed %s\n", forget)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("give a Douyin url, or run dyt --list")
	}
	if speech != "" {
		if extra, ok := speechFile(speech); ok {
			found.Speech = append(found.Speech, extra)
		}
	}
	localSpeech := speech
	if speechURL != "" {
		localSpeech = ""
	}
	choice, err := models.Pick(found, models.Request{
		Speech: localSpeech,
		Chat:   chat,
		Polish: !noPolish,
	})
	if err != nil {
		printModels(found)
		return err
	}
	if speechURL != "" {
		choice.Speech = remoteSpeech(speechURL, speech, key)
	}
	fmt.Fprintf(os.Stderr, "Speech: %s\n", choice.Speech)
	if choice.Polish {
		fmt.Fprintf(os.Stderr, "Polish: %s\n", choice.Chat)
	}

	if err := downloader.CheckDependencies(); err != nil {
		return err
	}
	if choice.Speech.Path != "" {
		if err := transcriber.CheckLocalDependencies(); err != nil {
			return err
		}
	}

	inputURL := args[0]

	// Resolve URL
	fmt.Fprintf(os.Stderr, "Resolving URL...\n")
	videoURL, err := resolver.Resolve(inputURL)
	if err != nil {
		return fmt.Errorf("resolving URL: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Resolved: %s\n", videoURL)

	// Download audio
	videoID := idFromURL(videoURL)
	speechName := choice.Speech.Ref
	chatName := ""
	if choice.Polish {
		chatName = choice.Chat.Ref
	}
	if cached, ok := cache.Load(videoID, speechName, chatName); ok {
		fmt.Fprintf(os.Stderr, "Using cached transcript: %s\n", videoID)
		text := cached.Text
		return writeResult(text)
	}

	fmt.Fprintf(os.Stderr, "Downloading audio...\n")
	audioPath, videoTitle, err := downloader.DownloadAudio(videoURL, "")
	if err != nil {
		return fmt.Errorf("downloading audio: %w", err)
	}
	if keepAudio {
		fmt.Fprintf(os.Stderr, "Audio saved: %s\n", audioPath)
	}
	fmt.Fprintf(os.Stderr, "Audio downloaded: %s\n", audioPath)

	fmt.Fprintf(os.Stderr, "Transcribing...\n")
	var text string
	if choice.Speech.Path != "" {
		text, err = transcriber.TranscribeLocal(audioPath, transcriber.LocalOptions{
			ModelPath: choice.Speech.Path,
			Language:  language,
		})
	} else {
		text, err = transcriber.Transcribe(audioPath, transcriber.Options{
			APIKey:   choice.Speech.APIKey,
			Endpoint: choice.Speech.BaseURL,
			Model:    choice.Speech.Name,
			Language: language,
			Prompt:   prompt,
		})
	}
	if err != nil {
		return fmt.Errorf("transcribing: %w", err)
	}

	if choice.Polish {
		if rawOutput != "" {
			if err := os.WriteFile(rawOutput, []byte(text+"\n"), 0644); err != nil {
				return fmt.Errorf("writing raw transcript: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Raw transcript saved to: %s\n", rawOutput)
		}
		candidates, err := models.ChatCandidates(found, chat, models.MagpieFirst)
		if err != nil {
			return err
		}
		var last error
		for i, candidate := range candidates {
			if i > 0 {
				wait := models.Backoff(i)
				warn("backing off %s, next model %s", wait, candidate.Ref)
				time.Sleep(wait)
			}
			fmt.Fprintf(os.Stderr, "Polishing with %s...\n", candidate)
			polished, err := polish.Chat(text, polish.Options{
				BaseURL: candidate.BaseURL,
				Model:   candidate.Name,
				APIKey:  candidate.APIKey,
			})
			if err == nil {
				fmt.Fprintf(os.Stderr, "Polished with %s\n", candidate.Ref)
				text = polished.Text
				last = nil
				break
			}
			last = err
			warn("%s failed: %s", candidate.Ref, oneLine(err.Error()))
		}
		if last != nil {
			return fmt.Errorf("polishing: %w", last)
		}
	}

	if err := cache.Save(cache.Record{
		ID: videoID, Title: videoTitle, URL: videoURL, Input: inputURL,
		Text: text, Speech: speechName, Chat: chatName,
	}); err != nil {
		warn("cache: %s", err)
	}
	return writeResult(text)
}

func writeResult(text string) error {
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

func remoteSpeech(base, name, key string) models.Model {
	if name == "" {
		name = "whisper-1"
	}
	return models.Model{
		Kind: models.KindSpeech, Ref: name, Name: name,
		BaseURL: base, APIKey: key, Where: "speech-url",
	}
}

func speechFile(value string) (models.Model, bool) {
	if !strings.Contains(value, "/") && !strings.HasSuffix(value, ".bin") {
		return models.Model{}, false
	}
	info, err := os.Stat(value)
	if err != nil || info.Size() < 1024*1024 {
		return models.Model{}, false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(value), "ggml-"), ".bin")
	return models.Model{
		Kind: models.KindSpeech, Ref: value, Name: name, Path: value, Where: "whisper.cpp",
	}, true
}

func printHistory() error {
	list, err := cache.List()
	if err != nil {
		return err
	}
	if list == nil {
		list = []cache.Record{}
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func idFromURL(videoURL string) string {
	parts := strings.Split(strings.TrimRight(videoURL, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}

func printModels(set models.Set) {
	fmt.Fprintln(os.Stderr, "Speech models:")
	if len(set.Speech) == 0 {
		fmt.Fprintln(os.Stderr, "  (none)")
	}
	for _, m := range set.Speech {
		fmt.Fprintf(os.Stderr, "  %s\n", m)
	}
	fmt.Fprintln(os.Stderr, "Chat models:")
	if len(set.Chat) == 0 {
		fmt.Fprintln(os.Stderr, "  (none)")
	}
	for _, m := range set.Chat {
		fmt.Fprintf(os.Stderr, "  %s\n", m)
	}
}
