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

# Discover models, hear with the best local whisper file, rewrite with Magpie
dyt -l zh "https://v.douyin.com/abc123/"

# See what was found
dyt --list

# Name the two models separately
dyt --speech small "https://v.douyin.com/abc123/"
dyt --chat llama3.1:8b "https://v.douyin.com/abc123/"
dyt --speech ~/.local/share/whisper-cpp/ggml-large-v3-turbo.bin --chat llama3.1:8b "https://v.douyin.com/abc123/"

# A remote speech service. This is /audio/transcriptions, not the chat /v1
dyt --speech-url https://api.example.com/v1 "https://v.douyin.com/abc123/"
dyt --speech whisper-1 --speech-url https://api.example.com/v1 --chat llama3.1:8b "https://v.douyin.com/abc123/"

# Transcribe only
dyt --no-polish "https://v.douyin.com/abc123/"

# Also look for chat models at one OpenAI-compatible catalog
dyt --base-url "https://api.example.com/v1" "https://v.douyin.com/abc123/"

# Save to file
dyt -o transcript.txt "https://v.douyin.com/abc123/"

# Keep the downloaded audio
dyt --keep-audio "https://v.douyin.com/abc123/"
```

### 模型怎么选

`dyt` 先列出找得到的模型，再各选一个。`--model` 只写名字，不写它在哪。

找得到的地方：

- 语音：`~/.local/share/whisper-cpp`、`~/.dyt/models`、`~/.cache/whisper` 里的 `ggml-*.bin`。这是 whisper.cpp 的模型文件，不是 OpenAI 兼容接口
- 润色：Magpie（`127.0.0.1:3425`）、Ollama（`127.0.0.1:11434`），以及 `--base-url` 列出的对话模型

没写 `--speech` 时，语音用最大的本地 ggml 文件。润色用 Magpie 的第一个模型，Magpie 没开就用 Ollama，再没有就用 `--base-url`。

两个模型分开配置。

- `--speech` 是本地 ggml 的名字或路径。加上 `--speech-url` 之后，同一个 `--speech` 变成远端语音服务上的模型名，默认 `whisper-1`。这个地址要接受 `/v1/audio/transcriptions`。
- `--chat` 只看 Magpie、Ollama 和 `--base-url` 上的对话模型。

不写的那一边按优先级选。远端语音服务不会被当成润色模型。

`--no-polish` 只做语音转写。`--list` 只打印找到的模型。

`--raw-output` 另外留下润色前的识别稿。whisper.cpp 的线程数由它自己决定，命令行不再透出。

## How It Works

1. **Resolve** — short URL → follow redirects → extract video ID
2. **Fetch** — get video info from Douyin's mobile web page (no API key/cookies needed)
3. **Download** — grab audio from CDN directly
4. **Discover** — 找出本机 ggml 语音模型，以及 Magpie、Ollama、`--base-url` 上的润色模型
5. **Transcribe** — 选中的语音模型
6. **Polish** — 选中的润色模型。`--no-polish` 跳过

## License

MIT
