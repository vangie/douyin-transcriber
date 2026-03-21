# douyin-transcriber (dyt)

CLI tool to transcribe Douyin (抖音) video speech to text.

## Prerequisites

- [yt-dlp](https://github.com/yt-dlp/yt-dlp): `brew install yt-dlp`
- [ffmpeg](https://ffmpeg.org/): `brew install ffmpeg`
- [OpenAI API key](https://platform.openai.com/api-keys)

## Install

### From source

```bash
go install github.com/babelcloud/douyin-transcriber@latest
```

### From releases

Download the binary for your platform from [Releases](https://github.com/babelcloud/douyin-transcriber/releases).

### Build from source

```bash
git clone https://github.com/babelcloud/douyin-transcriber.git
cd douyin-transcriber
make build
```

## Usage

```bash
# Set your OpenAI API key
export OPENAI_API_KEY=sk-...

# Transcribe a Douyin video
dyt "https://v.douyin.com/abc123/"

# Save transcript to file
dyt -o transcript.txt "https://v.douyin.com/abc123/"

# Specify language hint for better accuracy
dyt -l zh "https://v.douyin.com/abc123/"

# Use a custom Whisper API endpoint
dyt --endpoint "https://your-api.com/v1/audio/transcriptions" "https://v.douyin.com/abc123/"

# Keep the downloaded audio file
dyt --keep-audio "https://v.douyin.com/abc123/"
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--api-key` | | OpenAI API key (or set `OPENAI_API_KEY` env var) |
| `--endpoint` | | Custom Whisper API endpoint |
| `--model` | | Whisper model (default: `whisper-1`) |
| `--language` | `-l` | Language hint (ISO 639-1: `zh`, `en`, etc.) |
| `--prompt` | | Context prompt to guide transcription |
| `--output` | `-o` | Write transcript to file |
| `--keep-audio` | | Keep downloaded audio file |

## Cross-platform Builds

Uses [GoReleaser](https://goreleaser.com/) for building cross-platform binaries:

```bash
# Build snapshot (local testing)
make release-snapshot

# Full release (requires git tag)
make release
```

Supported platforms: Linux, macOS, Windows (amd64, arm64).

## Development

```bash
make test          # Run all tests
make test-short    # Run tests (skip long-running)
make build         # Build binary
make lint          # Run linter
```

## License

MIT
