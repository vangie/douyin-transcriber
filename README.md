# Douyin Transcriber (dyt)

A CLI tool that transcribes Douyin (抖音) video speech to text.

## Features

- 🔗 **Smart URL resolution** — supports short links (`v.douyin.com`), full URLs, and pasted share text
- 📥 **Direct CDN download** — no yt-dlp needed, no cookies, no login required
- 🎙️ **Dual transcription modes**:
  - **Local mode** — whisper.cpp (free, private, offline)
  - **Cloud mode** — OpenAI Whisper API (faster, more accurate)
- 🌐 **Multi-language** — supports Chinese, English, and all languages whisper supports

## Installation

```bash
brew install vangie/formula/douyin-transcriber
```

### Prerequisites

- **ffmpeg** — `brew install ffmpeg` (audio extraction)
- **Local mode** — `brew install whisper-cpp` (auto-downloads models on first run)
- **Cloud mode** — `export OPENAI_API_KEY=sk-...`

## Usage

```bash
# Paste a Douyin share link — local mode (auto-detected when no API key)
dyt "https://v.douyin.com/n234HvHgzN0/"

# Full URL works too
dyt "https://www.douyin.com/video/7616660340146752819"

# Paste entire share text — URL is auto-extracted
dyt "8.46 复制打开抖音，看看【理性避难所的作品】第9集... https://v.douyin.com/n234HvHgzN0/ 05/31"

# Force local mode with language hint
dyt --local -l zh "https://v.douyin.com/abc123/"

# Cloud mode with custom endpoint (e.g., compatible API)
dyt --cloud --endpoint "https://api.example.com/v1" "https://v.douyin.com/abc123/"

# Save to file
dyt -o transcript.txt "https://v.douyin.com/abc123/"

# Keep the downloaded audio
dyt --keep-audio "https://v.douyin.com/abc123/"
```

### Mode Auto-Detection

| Condition | Mode |
|-----------|------|
| `OPENAI_API_KEY` set | Cloud (OpenAI Whisper API) |
| `OPENAI_API_KEY` not set | Local (whisper.cpp) |
| `--local` flag | Force local |
| `--cloud` flag | Force cloud |

### Local Mode Options

```bash
# Use a different model (tiny/base/small/medium/large-v3)
dyt --model-name small -l zh "URL"

# Use a specific model file
dyt --model-path /path/to/ggml-large-v3.bin "URL"

# Control thread count
dyt -t 8 "URL"
```

## How It Works

1. **Resolve** — short URL → follow redirects → extract video ID
2. **Fetch** — get video info from Douyin's mobile web page (no API key/cookies needed)
3. **Download** — grab audio from CDN directly
4. **Transcribe** — whisper.cpp (local) or OpenAI API (cloud)

## License

MIT
