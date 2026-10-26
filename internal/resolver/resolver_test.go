package resolver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsDouyinURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"short URL", "https://v.douyin.com/abc123/", true},
		{"full URL", "https://www.douyin.com/video/123456", true},
		{"iesdouyin", "https://www.iesdouyin.com/share/video/123", true},
		{"not douyin", "https://www.youtube.com/watch?v=abc", false},
		{"empty", "", false},
		{"just text", "hello world", false},
		{"case insensitive", "HTTPS://WWW.DOUYIN.COM/video/123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDouyinURL(tt.url)
			if got != tt.want {
				t.Errorf("IsDouyinURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractURL(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			"short URL in text",
			"Check this out https://v.douyin.com/abc123/ cool right?",
			"https://v.douyin.com/abc123/",
		},
		{
			"full URL",
			"https://www.douyin.com/video/7338729384926",
			"https://www.douyin.com/video/7338729384926",
		},
		{
			"no URL",
			"just some text",
			"",
		},
		{
			"Chinese text with URL",
			"看看这个视频 https://v.douyin.com/n234HvHgzN0/ 太搞笑了",
			"https://v.douyin.com/n234HvHgzN0/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURL(tt.text)
			if got != tt.want {
				t.Errorf("ExtractURL(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestResolve_EmptyURL(t *testing.T) {
	_, err := Resolve("")
	if err == nil {
		t.Error("Resolve(\"\") should return error")
	}
}

func TestResolve_NotDouyin(t *testing.T) {
	_, err := Resolve("https://www.youtube.com/watch?v=abc")
	if err == nil {
		t.Error("Resolve(youtube) should return error")
	}
}

func TestResolve_FullDouyinURL(t *testing.T) {
	url := "https://www.douyin.com/video/7338729384926"
	got, err := Resolve(url)
	if err != nil {
		t.Fatalf("Resolve(%q) error: %v", url, err)
	}
	if got != url {
		t.Errorf("Resolve(%q) = %q, want %q", url, got, url)
	}
}

func TestResolve_NormalizesIesdouyin(t *testing.T) {
	input := "https://www.iesdouyin.com/share/video/7616660340146752819"
	expected := "https://www.douyin.com/video/7616660340146752819"
	got, err := Resolve(input)
	if err != nil {
		t.Fatalf("Resolve(%q) error: %v", input, err)
	}
	if got != expected {
		t.Errorf("Resolve(%q) = %q, want %q", input, got, expected)
	}
}

func TestResolve_ShortURLRedirect(t *testing.T) {
	finalURL := ""
	mux := http.NewServeMux()
	var ts *httptest.Server

	mux.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalURL, http.StatusFound)
	})
	mux.HandleFunc("/video/123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts = httptest.NewServer(mux)
	defer ts.Close()
	finalURL = ts.URL + "/video/123"

	resolved, err := resolveRedirects(ts.URL + "/short")
	if err != nil {
		t.Fatalf("resolveRedirects error: %v", err)
	}
	if resolved != finalURL {
		t.Errorf("resolveRedirects = %q, want %q", resolved, finalURL)
	}
}

func TestResolve_ExtractFromText(t *testing.T) {
	expected := "https://www.douyin.com/video/7338729384926"
	text := "Check this: " + expected + " nice!"
	got, err := Resolve(text)
	if err != nil {
		t.Fatalf("Resolve(%q) error: %v", text, err)
	}
	if got != expected {
		t.Errorf("Resolve(%q) = %q, want %q", text, got, expected)
	}
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"standard", "https://www.douyin.com/video/7616660340146752819", "7616660340146752819"},
		{"share path", "https://www.iesdouyin.com/share/video/7616660340146752819?region=CN", "7616660340146752819"},
		{"with query", "https://www.douyin.com/video/123?extra=stuff", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractVideoID(tt.url)
			if err != nil {
				t.Fatalf("extractVideoID(%q) error: %v", tt.url, err)
			}
			if got != tt.want {
				t.Errorf("extractVideoID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestResolveVideoID(t *testing.T) {
	videoID, err := ResolveVideoID("https://www.douyin.com/video/7616660340146752819")
	if err != nil {
		t.Fatalf("ResolveVideoID error: %v", err)
	}
	if videoID != "7616660340146752819" {
		t.Errorf("ResolveVideoID = %q, want 7616660340146752819", videoID)
	}
}
