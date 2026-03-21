// Package resolver handles Douyin URL resolution.
// It supports short URLs (v.douyin.com), full URLs (www.douyin.com/video/ID),
// and extracts Douyin URLs from text.
package resolver

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	// douyinURLPattern matches various Douyin URL formats.
	douyinURLPattern = regexp.MustCompile(`https?://(?:v\.douyin\.com|www\.douyin\.com|m\.douyin\.com|iesdouyin\.com)/[^\s"'<>]+`)
	// videoIDPattern extracts a video ID from various path formats.
	videoIDPattern = regexp.MustCompile(`(?:/share)?/video/(\d+)`)
)

// IsDouyinURL checks if the given string contains a Douyin URL.
func IsDouyinURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "douyin.com/")
}

// ExtractURL finds a Douyin URL in the given text.
func ExtractURL(text string) string {
	match := douyinURLPattern.FindString(text)
	return match
}

// Resolve takes a Douyin URL (or text containing one) and returns
// a canonical video URL: https://www.douyin.com/video/<ID>
func Resolve(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty URL")
	}

	rawURL := input
	if !strings.HasPrefix(input, "http") {
		extracted := ExtractURL(input)
		if extracted == "" {
			return "", fmt.Errorf("no Douyin URL found in input")
		}
		rawURL = extracted
	}

	if !IsDouyinURL(rawURL) {
		return "", fmt.Errorf("not a Douyin URL: %s", rawURL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())

	// Short URL: resolve redirects first
	if strings.Contains(host, "v.douyin.com") {
		resolved, err := resolveRedirects(rawURL)
		if err != nil {
			return "", fmt.Errorf("resolving short URL: %w", err)
		}
		rawURL = resolved
	}

	// Normalize to canonical format
	return normalizeURL(rawURL)
}

// ResolveVideoID extracts just the video ID from any Douyin URL.
func ResolveVideoID(input string) (string, error) {
	resolved, err := Resolve(input)
	if err != nil {
		return "", err
	}
	return extractVideoID(resolved)
}

func resolveRedirects(shortURL string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", shortURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("following redirects: %w", err)
	}
	defer resp.Body.Close()

	return resp.Request.URL.String(), nil
}

// normalizeURL converts iesdouyin.com/share/video/ID or m.douyin.com/... etc
// to the canonical https://www.douyin.com/video/<ID>
func normalizeURL(rawURL string) (string, error) {
	videoID, err := extractVideoID(rawURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://www.douyin.com/video/%s", videoID), nil
}

func extractVideoID(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}

	// Path-based: /video/ID or /share/video/ID
	matches := videoIDPattern.FindStringSubmatch(parsed.Path)
	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Query parameter: ?modal_id=ID
	if modalID := parsed.Query().Get("modal_id"); modalID != "" {
		return modalID, nil
	}

	return "", fmt.Errorf("cannot extract video ID from URL: %s", rawURL)
}
