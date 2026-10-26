package models

import "testing"

func TestPickUsesPriorityWhenNothingIsNamed(t *testing.T) {
	set := Set{
		Speech: []Model{
			{Kind: KindSpeech, Ref: "base", Name: "base", Path: "/m/ggml-base.bin", Where: "whisper.cpp"},
			{Kind: KindSpeech, Ref: "large-v3-turbo", Name: "large-v3-turbo", Path: "/m/ggml-large-v3-turbo.bin", Where: "whisper.cpp"},
		},
		Chat: []Model{
			{Kind: KindChat, Ref: "llama3.1:8b", Name: "llama3.1:8b", Where: "ollama"},
			{Kind: KindChat, Ref: "gemini/gemini-3.5-flash", Name: "gemini/gemini-3.5-flash", Where: "magpie"},
		},
	}
	got, err := Pick(set, Request{Polish: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Speech.Ref != "large-v3-turbo" {
		t.Fatalf("speech %s", got.Speech.Ref)
	}
	if got.Chat.Where != "magpie" {
		t.Fatalf("chat %s", got.Chat.Where)
	}
}

func TestSpeechAndChatAreNamedSeparately(t *testing.T) {
	set := Set{
		Speech: []Model{
			{Kind: KindSpeech, Ref: "large-v3-turbo", Name: "large-v3-turbo", Path: "/m/a.bin", Where: "whisper.cpp"},
			{Kind: KindSpeech, Ref: "small", Name: "small", Path: "/m/b.bin", Where: "whisper.cpp"},
		},
		Chat: []Model{
			{Kind: KindChat, Ref: "llama3.1:8b", Where: "ollama"},
			{Kind: KindChat, Ref: "org/model", BaseURL: "https://api.example.com/v1", Where: "openai"},
		},
	}
	got, err := Pick(set, Request{Speech: "small", Chat: "org/model", Polish: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Speech.Ref != "small" || got.Speech.Where != "whisper.cpp" {
		t.Fatalf("speech %+v", got.Speech)
	}
	if got.Chat.Ref != "org/model" || got.Chat.Where != "openai" {
		t.Fatalf("chat %+v", got.Chat)
	}
}

func TestNamedSpeechKeepsChatPriority(t *testing.T) {
	set := Set{
		Speech: []Model{
			{Kind: KindSpeech, Ref: "large-v3-turbo", Path: "/m/a.bin", Where: "whisper.cpp"},
			{Kind: KindSpeech, Ref: "small", Path: "/m/b.bin", Where: "whisper.cpp"},
		},
		Chat: []Model{
			{Kind: KindChat, Ref: "llama3.1:8b", Where: "ollama"},
			{Kind: KindChat, Ref: "gemini/gemini-3.5-flash", Where: "magpie"},
		},
	}
	got, err := Pick(set, Request{Speech: "small", Polish: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Speech.Ref != "small" {
		t.Fatalf("speech %s", got.Speech.Ref)
	}
	if got.Chat.Where != "magpie" {
		t.Fatalf("chat %s", got.Chat.Where)
	}
}

func TestMissingName(t *testing.T) {
	set := Set{Speech: []Model{{Kind: KindSpeech, Ref: "base", Path: "/m/a.bin"}}}
	if _, err := Pick(set, Request{Speech: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNoPolishSkipsChat(t *testing.T) {
	set := Set{Speech: []Model{{Kind: KindSpeech, Ref: "base", Name: "base", Path: "/m/a.bin", Where: "whisper.cpp"}}}
	got, err := Pick(set, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Polish || got.Chat.Ref != "" {
		t.Fatalf("%+v", got)
	}
}
