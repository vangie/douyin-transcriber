package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDependencies(t *testing.T) {
	err := CheckDependencies()
	if err != nil {
		t.Skipf("Dependencies not installed: %v", err)
	}
}

func TestExtractVideoIDFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.douyin.com/video/7616660340146752819", "7616660340146752819"},
		{"https://www.douyin.com/video/123/", "123"},
	}
	for _, tt := range tests {
		got, err := extractVideoIDFromURL(tt.url)
		if err != nil {
			t.Fatalf("extractVideoIDFromURL(%q) error: %v", tt.url, err)
		}
		if got != tt.want {
			t.Errorf("extractVideoIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestCleanup(t *testing.T) {
	dir, err := os.MkdirTemp("", "dyt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	audioPath := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(audioPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	Cleanup(audioPath)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Cleanup should have removed %s", dir)
		os.RemoveAll(dir)
	}
}

func TestFetchVideoInfoFromFeed(t *testing.T) {
	const body = `{
	  "status_code": 0,
	  "aweme_list": [
	    {"aweme_id": "1", "desc": "other", "video": {"play_addr": {"url_list": ["https://cdn.example/a.mp4"]}}},
	    {"aweme_id": "7700606818350796082", "desc": "盘点一周AI大事", "video": {
	      "play_addr": {"url_list": ["https://cdn.example/play.mp4"]},
	      "play_addr_h264": {"url_list": ["https://cdn.example/h264.mp4"]}
	    }}
	  ]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "aweme_id=7700606818350796082") {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	old := feedBase
	feedBase = srv.URL
	t.Cleanup(func() { feedBase = old })

	info, err := FetchVideoInfo("7700606818350796082")
	if err != nil {
		t.Fatal(err)
	}
	if info.VideoURL != "https://cdn.example/h264.mp4" || info.Title != "盘点一周AI大事" {
		t.Fatalf("%+v", info)
	}
}

func TestCleanup_NonTempDir(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(audioPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	Cleanup(audioPath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Cleanup should not have removed non-temp dir")
	}
}
