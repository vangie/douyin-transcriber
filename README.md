# douyin-transcriber (dyt)

CLI tool to transcribe Douyin (抖音) video speech to text.

**Smart mode auto-detection**: Just run `dyt <url>` — it picks the right mode automatically:
- `OPENAI_API_KEY` set → cloud mode (OpenAI Whisper API)
- `OPENAI_API_KEY` not set → local mode (whisper.cpp, no API key needed)

Override with `--cloud` or `--local` flags when needed.

## Prerequisites

- [yt-dlp](https://github.com/yt-dlp/yt-dlp): `brew install yt-dlp`
- [ffmpeg](https://ffmpeg.org/): `brew install ffmpeg`

For cloud mode:
- [OpenAI API key](https://platform.openai.com/api-keys)

For local mode:
- [whisper.cpp](https://github.com/ggerganov/whisper.cpp): `brew install whisper-cpp`

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

### Quick start (auto-detect mode)

```bash
# No API key? Runs locally with whisper.cpp (model auto-downloads)
dyt "https://v.douyin.com/abc123/"

# Have an API key? Uses cloud mode automatically
export OPENAI_API_KEY=sk-...
dyt "https://v.douyin.com/abc123/"
```

### Cloud mode (OpenAI Whisper API)

```bash
export OPENAI_API_KEY=sk-...

# Basic usage (auto-detected)
dyt "https://v.douyin.com/abc123/"

# Force cloud mode explicitly
dyt --cloud "https://v.douyin.com/abc123/"

# With language hint
dyt -l zh "https://v.douyin.com/abc123/"

# Save to file
dyt -o transcript.txt "https://v.douyin.com/abc123/"

# Custom API endpoint (compatible APIs)
dyt --endpoint "https://your-api.com/v1/audio/transcriptions" "https://v.douyin.com/abc123/"
```

### Local mode (whisper.cpp, no API key needed)

```bash
# Auto-detected when no OPENAI_API_KEY is set
dyt "https://v.douyin.com/abc123/"

# Force local mode (even if API key is set)
dyt --local "https://v.douyin.com/abc123/"

# Specify model size (tiny/base/small/medium/large-v3)
dyt --local --model-name large-v3 "https://v.douyin.com/abc123/"

# Use your own model file
dyt --local --model-path /path/to/ggml-model.bin "https://v.douyin.com/abc123/"

# With language hint and thread count
dyt --local -l zh -t 8 "https://v.douyin.com/abc123/"
```

### Mode flags

| Flag | Description |
|------|-------------|
| `--local` | Force local whisper.cpp mode (even if API key is set) |
| `--cloud` | Force cloud mode (requires `OPENAI_API_KEY`) |

> Without either flag, dyt auto-detects: API key present → cloud, absent → local.

### Common flags

| Flag | Short | Description |
|------|-------|-------------|
| `--language` | `-l` | Language hint (ISO 639-1: `zh`, `en`, etc.) |
| `--output` | `-o` | Write transcript to file |
| `--keep-audio` | | Keep downloaded audio file |

### Cloud mode flags

| Flag | Description |
|------|-------------|
| `--api-key` | OpenAI API key (or set `OPENAI_API_KEY` env var) |
| `--endpoint` | Custom Whisper API endpoint |
| `--model` | Whisper model (default: `whisper-1`) |
| `--prompt` | Context prompt to guide transcription |

### Local mode flags

| Flag | Short | Description |
|------|-------|-------------|
| `--model-name` | | Model to download: `tiny`, `base`, `small`, `medium`, `large-v3` (default: `base`) |
| `--model-path` | | Path to a local ggml model file |
| `--threads` | `-t` | Number of CPU threads (0 = auto) |

## Cross-platform Builds

Uses [GoReleaser](https://goreleaser.com/) for building cross-platform binaries:

```bash
make release-snapshot   # Local snapshot build
make release            # Full release (requires git tag)
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
