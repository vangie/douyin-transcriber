// Package models finds speech models and chat models, then picks one of each.
//
// Finding a model and choosing a model are separate. Discover talks to the
// local services and the filesystem. Pick applies one requested name, or the
// priority order when nothing was requested.
package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	KindSpeech = "speech"
	KindChat   = "chat"
)

// Model is one usable model. Ref is what --model matches.
type Model struct {
	Kind    string
	Ref     string
	Name    string
	BaseURL string // OpenAI-compatible base ending in /v1. Empty for a local file.
	Path    string // ggml file. Empty for a served model.
	APIKey  string
	Where   string // ollama, magpie, openai, whisper.cpp
}

func (m Model) String() string {
	if m.Path != "" {
		return m.Kind + " " + m.Ref + " (" + m.Path + ")"
	}
	return m.Kind + " " + m.Ref + " @ " + m.Where
}

// Set is everything Discover found.
type Set struct {
	Speech []Model
	Chat   []Model
}

// Discover looks at local ggml files, Magpie, Ollama, and an optional
// OpenAI-compatible base. A service that is down is skipped.
func Discover(opt DiscoverOptions) Set {
	var set Set
	for _, path := range localSpeechFiles() {
		name := speechName(filepath.Base(path))
		set.Speech = append(set.Speech, Model{
			Kind: KindSpeech, Ref: name, Name: name, Path: path, Where: "whisper.cpp",
		})
	}
	client := &http.Client{Timeout: 3 * time.Second}
	if names := listServed(client, "http://127.0.0.1:3425/v1/models"); len(names) > 0 {
		for _, name := range names {
			set.Chat = append(set.Chat, Model{
				Kind: KindChat, Ref: name, Name: name,
				BaseURL: "http://127.0.0.1:3425/v1", Where: "magpie",
			})
		}
	}
	if names := listServed(client, "http://127.0.0.1:11434/v1/models"); len(names) > 0 {
		for _, name := range names {
			set.Chat = append(set.Chat, Model{
				Kind: KindChat, Ref: name, Name: name,
				BaseURL: "http://127.0.0.1:11434/v1", Where: "ollama",
			})
		}
	}
	if base := strings.TrimSpace(opt.BaseURL); base != "" {
		base = strings.TrimRight(base, "/")
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
		for _, name := range listServed(client, base+"/models") {
			// A /v1 catalog lists chat models. Speech models are local ggml files.
			set.Chat = append(set.Chat, Model{
				Kind: KindChat, Ref: name, Name: name, BaseURL: base, APIKey: opt.APIKey, Where: "openai",
			})
		}
	}
	return set
}

// DiscoverOptions adds one remote catalog to the local scan.
type DiscoverOptions struct {
	BaseURL string
	APIKey  string
}

// Choice is the speech model and the chat model for one run.
type Choice struct {
	Speech Model
	Chat   Model
	Polish bool
}

// Request is the one place a caller names models. Empty fields stay on priority.
// Speech is matched only against speech models, so naming a local ggml file
// does not change which chat model is used.
type Request struct {
	Speech string
	Chat   string
	Polish bool
}

// Pick selects one speech model and, when polish is set, one chat model.
func Pick(set Set, req Request) (Choice, error) {
	speechHits := match(set.Speech, req.Speech)
	if req.Speech != "" && len(speechHits) == 0 {
		return Choice{}, fmt.Errorf("speech model %q was not found", req.Speech)
	}
	chatHits := match(set.Chat, req.Chat)
	if req.Chat != "" && len(chatHits) == 0 {
		return Choice{}, fmt.Errorf("chat model %q was not found", req.Chat)
	}
	speech, err := first(orHits(speechHits, set.Speech), speechRank, "speech model")
	if err != nil {
		return Choice{}, err
	}
	choice := Choice{Speech: speech, Polish: req.Polish}
	if !req.Polish {
		return choice, nil
	}
	chat, err := first(orHits(chatHits, set.Chat), chatRank, "chat model")
	if err != nil {
		return Choice{}, err
	}
	choice.Chat = chat
	return choice, nil
}

func match(list []Model, want string) []Model {
	if want == "" {
		return nil
	}
	var hits []Model
	for _, m := range list {
		if m.Ref == want || m.Name == want || samePath(m.Path, want) || (m.Path != "" && filepath.Base(m.Path) == want) {
			hits = append(hits, m)
		}
	}
	return hits
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return aa == bb
}

func orHits(hits, all []Model) []Model {
	if len(hits) > 0 {
		return hits
	}
	return all
}

func first(list []Model, rank func(Model) int, what string) (Model, error) {
	if len(list) == 0 {
		return Model{}, fmt.Errorf("%s: none found", what)
	}
	return best(list, rank), nil
}

func best(list []Model, rank func(Model) int) Model {
	top := list[0]
	for _, m := range list[1:] {
		if rank(m) < rank(top) {
			top = m
		}
	}
	return top
}

// Larger local files rank first. A served speech model is only used when no
// local file exists, or when it was requested by name.
func speechRank(m Model) int {
	if m.Path == "" {
		return 1000
	}
	switch {
	case strings.Contains(m.Name, "large-v3-turbo"):
		return 0
	case strings.Contains(m.Name, "large-v3"):
		return 1
	case strings.Contains(m.Name, "large"):
		return 2
	case strings.Contains(m.Name, "medium"):
		return 3
	case strings.Contains(m.Name, "small"):
		return 4
	case strings.Contains(m.Name, "base"):
		return 5
	default:
		return 6
	}
}

// ChatOrder is which provider is tried first when no single model is named.
// The other providers follow, one model each.
type ChatOrder string

const (
	MagpieFirst ChatOrder = "magpie-first"
	OllamaFirst ChatOrder = "ollama-first"
)

// ChatCandidates is the polish order. A model name returns that model alone.
// An order returns one model per provider. A signed-out provider then fails
// once and the next provider is tried.
func ChatCandidates(set Set, want string, order ChatOrder) ([]Model, error) {
	if want != "" && want != string(MagpieFirst) && want != string(OllamaFirst) {
		hits := match(set.Chat, want)
		if len(hits) == 0 {
			return nil, fmt.Errorf("chat model %q was not found", want)
		}
		return []Model{best(hits, chatRank)}, nil
	}
	if want == string(OllamaFirst) {
		order = OllamaFirst
	}
	if order == "" {
		order = MagpieFirst
	}
	if len(set.Chat) == 0 {
		return nil, fmt.Errorf("chat model: none found")
	}
	list := append([]Model(nil), set.Chat...)
	sort.SliceStable(list, func(i, j int) bool {
		return providerRank(list[i], order) < providerRank(list[j], order)
	})
	return onePerProvider(list), nil
}

func providerRank(m Model, order ChatOrder) int {
	rank := chatRank(m)
	if order == OllamaFirst {
		switch m.Where {
		case "ollama":
			return 0
		case "magpie":
			return 1
		default:
			return 2
		}
	}
	return rank
}

func onePerProvider(list []Model) []Model {
	seen := map[string]bool{}
	var out []Model
	for _, m := range list {
		if seen[m.Where] {
			continue
		}
		seen[m.Where] = true
		out = append(out, m)
	}
	return out
}

// Backoff is the wait before polish attempt n, starting at 1s and doubling
// until 8s. Attempt 1 is the first retry.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Second << (attempt - 1)
	if d > 8*time.Second {
		return 8 * time.Second
	}
	return d
}

// Magpie is the default provider. Ollama follows it.
func chatRank(m Model) int {
	switch m.Where {
	case "magpie":
		return 0
	case "ollama":
		return 1
	default:
		return 2
	}
}

func localSpeechFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dirs := []string{
		filepath.Join(home, ".local", "share", "whisper-cpp"),
		filepath.Join(home, ".dyt", "models"),
		filepath.Join(home, ".cache", "whisper"),
	}
	var paths []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "ggml-") || !strings.HasSuffix(name, ".bin") {
				continue
			}
			if strings.Contains(name, "for-tests") {
				continue
			}
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err == nil && info.Size() > 1024*1024 {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func speechName(filename string) string {
	name := strings.TrimPrefix(filename, "ggml-")
	return strings.TrimSuffix(name, ".bin")
}

func listServed(client *http.Client, url string) []string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil
	}
	var doc struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, m := range doc.Data {
		if m.ID != "" && !seen[m.ID] {
			seen[m.ID] = true
			names = append(names, m.ID)
		}
	}
	for _, m := range doc.Models {
		if m.Name != "" && !seen[m.Name] {
			seen[m.Name] = true
			names = append(names, m.Name)
		}
	}
	return names
}
