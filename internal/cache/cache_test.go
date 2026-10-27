package cache

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadAndExpiry(t *testing.T) {
	Root = t.TempDir()
	rec := Record{ID: "123", Title: "标题", URL: "https://www.douyin.com/video/123", Input: "share", Text: "正文", Speech: "large-v3-turbo", Chat: "llama3.1:8b"}
	if err := Save(rec); err != nil {
		t.Fatal(err)
	}
	got, ok := Load("123", "large-v3-turbo", "llama3.1:8b")
	if !ok || got.Text != "正文" {
		t.Fatalf("%v %+v", ok, got)
	}
	if _, ok := Load("123", "small", "llama3.1:8b"); ok {
		t.Fatal("different speech model should miss")
	}

	old := filepath.Join(Dir("123"), fileName("large-v3-turbo", "llama3.1:8b")+".json")
	past := time.Now().Add(-TTL - time.Hour)
	if err := osChtimes(old, past); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load("123", "large-v3-turbo", "llama3.1:8b"); ok {
		t.Fatal("expired record was reused")
	}
}

func TestListNewestPerVideo(t *testing.T) {
	Root = t.TempDir()
	first := Record{ID: "1", Text: "旧", Speech: "a", Chat: "b", At: time.Now().Add(-time.Hour)}
	second := Record{ID: "1", Text: "新", Speech: "c", Chat: "d", At: time.Now()}
	other := Record{ID: "2", Text: "另一条", Speech: "a", Chat: "b", At: time.Now().Add(-time.Minute)}
	for _, rec := range []Record{first, second, other} {
		if err := Save(rec); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "1" || list[0].Text != "新" || list[1].ID != "2" {
		t.Fatalf("%+v", list)
	}
}

func TestRemove(t *testing.T) {
	Root = t.TempDir()
	if err := Save(Record{ID: "9", Text: "正文", Speech: "a", Chat: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := Remove("9"); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load("9", "a", "b"); ok {
		t.Fatal("removed record was still readable")
	}
	if err := Remove("../etc"); err == nil {
		t.Fatal("path outside the cache was accepted")
	}
}

func TestFreshRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "video.mp4")
	if err := osWrite(path, nil); err != nil {
		t.Fatal(err)
	}
	if Fresh(path) {
		t.Fatal("empty file is not a cache hit")
	}
}

func osChtimes(path string, at time.Time) error {
	return chtimes(path, at)
}

func osWrite(path string, data []byte) error {
	return writeFile(path, data)
}
