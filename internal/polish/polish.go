// Package polish rewrites a whisper transcript through a local chat model.
//
// Ollama listens on 127.0.0.1:11434. Magpie, the local app, exposes an
// OpenAI-compatible API on 127.0.0.1:3425. Both accept /v1/chat/completions.
package polish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ollamaBase = "http://127.0.0.1:11434/v1"
	magpieBase = "http://127.0.0.1:3425/v1"
)

const (
	ollamaModel = "llama3.1:8b"
	magpieModel = "gemini/gemini-3.5-flash"
)

// Backend is "ollama" or "magpie".
type Backend string

const (
	Ollama Backend = "ollama"
	Magpie Backend = "magpie"
)

// Options selects the local chat endpoint.
type Options struct {
	Backend Backend
	Model   string
	BaseURL string
	// Title is the video description, used only as a hint for names.
	Title string
	// APIKey is sent as a bearer token. Local backends leave it empty.
	APIKey string
}

// Result is the rewritten transcript.
type Result struct {
	Text    string
	Model   string
	Backend Backend
}

const systemPrompt = `你在校对一段中文口播的语音识别稿。识别稿来自 whisper，同音字和标点经常错。

只做这些：
- 补上句号、逗号，把粘在一起的句子分开。
- 改正明显的同音错字，例如「马住」写成「mark 住」这种口语可以保留，不要改成另一种意思。
- 输出的字数和原稿接近。原稿有多少句，你就保留多少句。

不要做这些：
- 不要摘要，不要加标题，不要评价。
- 不要替换或发明专名。稿子里的英文名、产品名保持原样，哪怕你觉得它像另一个名字。
- 不要续写，不要补原稿没有的句子。
- 只输出校对后的正文。`

// Chat sends the transcript and returns the rewritten text.
// An empty Backend tries Magpie first and falls back to Ollama when Magpie
// is down or answers with an error.
func Chat(transcript string, opt Options) (Result, error) {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return Result{}, fmt.Errorf("empty transcript")
	}
	// A caller that already picked a model gets that one request. Fallback
	// belongs to the caller, which knows the rest of the catalog.
	if opt.BaseURL != "" && opt.Model != "" {
		return postChat(transcript, opt, opt.BaseURL, opt.Model)
	}
	if opt.Backend == "" {
		return chatAuto(transcript, opt)
	}
	return chatOne(transcript, opt)
}

func chatAuto(transcript string, opt Options) (Result, error) {
	opt.Backend = Magpie
	res, err := chatOne(transcript, opt)
	if err == nil {
		return res, nil
	}
	magpieErr := err
	opt.Backend = Ollama
	// The failed Magpie model name must not be sent to Ollama.
	opt.BaseURL = ""
	opt.Model = ""
	res, err = chatOne(transcript, opt)
	if err != nil {
		return Result{}, fmt.Errorf("magpie: %v; ollama: %v", magpieErr, err)
	}
	return res, nil
}

func chatOne(transcript string, opt Options) (Result, error) {
	if opt.Backend == "" && opt.BaseURL != "" && opt.Model != "" {
		return postChat(transcript, opt, opt.BaseURL, opt.Model)
	}
	base, model, err := resolve(opt)
	if err != nil {
		return Result{}, err
	}
	return postChat(transcript, opt, base, model)
}

func postChat(transcript string, opt Options, base, model string) (Result, error) {
	user := transcript
	if title := strings.TrimSpace(opt.Title); title != "" {
		user = "视频标题：" + title + "\n\n识别稿：\n" + transcript
	}
	body, err := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if opt.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+opt.APIKey)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", opt.Backend, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return Result{}, err
	}
	if res.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%s: HTTP %d: %s", opt.Backend, res.StatusCode, trim(raw, 400))
	}
	text, err := readCompletion(raw)
	if err != nil {
		return Result{}, err
	}
	where := opt.Backend
	if where == "" {
		where = whereOf(base)
	}
	return Result{Text: text, Model: model, Backend: where}, nil
}

func whereOf(base string) Backend {
	switch {
	case strings.Contains(base, "127.0.0.1:3425"):
		return Magpie
	case strings.Contains(base, "127.0.0.1:11434"):
		return Ollama
	default:
		return "openai"
	}
}

func resolve(opt Options) (base, model string, err error) {
	switch opt.Backend {
	case Ollama:
		base, model = ollamaBase, ollamaModel
	case Magpie:
		base, model = magpieBase, magpieModel
	case Backend("openai"):
		base, model = "https://api.openai.com/v1", ""
		if opt.BaseURL == "" || opt.Model == "" {
			return "", "", fmt.Errorf("--polish openai needs --base-url and --model")
		}
	default:
		return "", "", fmt.Errorf("unknown polish backend %q (use auto, ollama, magpie, or openai)", opt.Backend)
	}
	if opt.BaseURL != "" {
		base = opt.BaseURL
	}
	if opt.Model != "" {
		model = opt.Model
	}
	if !strings.HasSuffix(strings.TrimRight(base, "/"), "/v1") {
		base = strings.TrimRight(base, "/") + "/v1"
	}
	return base, model, nil
}

func readCompletion(raw []byte) (string, error) {
	var doc struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("chat json: %w", err)
	}
	if doc.Error.Message != "" {
		return "", fmt.Errorf("chat: %s", doc.Error.Message)
	}
	if len(doc.Choices) == 0 {
		return "", fmt.Errorf("chat: no choices")
	}
	text := strings.TrimSpace(doc.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("chat: empty content")
	}
	return text, nil
}

func trim(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n]
	}
	return s
}
