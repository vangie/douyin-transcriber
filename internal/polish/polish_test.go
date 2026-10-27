package polish

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChat(t *testing.T) {
	var gotModel, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path %s", r.URL.Path)
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		gotModel = req.Model
		gotUser = req.Messages[1].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"校对后的正文。"}}]}`))
	}))
	defer srv.Close()

	res, err := Chat("识别稿原文", Options{
		Backend: Ollama,
		Model:   "llama3.1:8b",
		BaseURL: srv.URL,
		Title:   "一周AI大事",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "校对后的正文。" {
		t.Fatal(res.Text)
	}
	if gotModel != "llama3.1:8b" {
		t.Fatal(gotModel)
	}
	if !strings.Contains(gotUser, "一周AI大事") || !strings.Contains(gotUser, "识别稿原文") {
		t.Fatal(gotUser)
	}
}

func TestResolveDefaults(t *testing.T) {
	base, model, err := resolve(Options{Backend: Magpie})
	if err != nil {
		t.Fatal(err)
	}
	if base != magpieBase || model != magpieModel {
		t.Fatalf("%s %s", base, model)
	}
	if _, _, err := resolve(Options{Backend: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAutoPrefersMagpie(t *testing.T) {
	magpie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"来自 magpie"}}]}`))
	}))
	defer magpie.Close()
	ollamaHits := 0
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaHits++
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"来自 ollama"}}]}`))
	}))
	defer ollama.Close()

	oldMagpie, oldOllama := magpieBase, ollamaBase
	magpieBase, ollamaBase = magpie.URL+"/v1", ollama.URL+"/v1"
	t.Cleanup(func() { magpieBase, ollamaBase = oldMagpie, oldOllama })

	res, err := Chat("原稿", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != Magpie || res.Text != "来自 magpie" || ollamaHits != 0 {
		t.Fatalf("%+v hits %d", res, ollamaHits)
	}
}

func TestAutoFallsBackToOllama(t *testing.T) {
	magpie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "signed out", http.StatusBadGateway)
	}))
	defer magpie.Close()
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"来自 ollama"}}]}`))
	}))
	defer ollama.Close()

	oldMagpie, oldOllama := magpieBase, ollamaBase
	magpieBase, ollamaBase = magpie.URL+"/v1", ollama.URL+"/v1"
	t.Cleanup(func() { magpieBase, ollamaBase = oldMagpie, oldOllama })

	res, err := Chat("原稿", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backend != Ollama || res.Text != "来自 ollama" {
		t.Fatalf("%+v", res)
	}
}

func TestEmpty(t *testing.T) {
	if _, err := Chat("  ", Options{Backend: Ollama}); err == nil {
		t.Fatal("expected error")
	}
}
