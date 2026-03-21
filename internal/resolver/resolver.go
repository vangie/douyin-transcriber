// Package resolver handles Douyin URL resolution.
// It resolves short URLs (v.douyin.com/xxx) to their final form.
package resolver

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var douyinShortURLPattern = regexp.MustCompile(`https?://v\.douyin\.com/\S+`)

// Resolve takes a Douyin URL (short or full) and resolves it to the final URL.
// Short URLs like https://v.douyin.com/xxx are followed via HTTP redirects.
func Resolve(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	// Extract URL if embedded in text
	if extracted := ExtractURL(rawURL); extracted != "" {
		rawURL = extracted
	}

	// Validate it looks like a Douyin URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.Contains(host, "douyin.com") {
		return "", fmt.Errorf("not a Douyin URL: %s", rawURL)
	}

	// If it's a short URL, resolve redirects
	if strings.Contains(host, "v.douyin.com") {
		return resolveRedirects(rawURL)
	}

	return rawURL, nil
}

// resolveRedirects follows HTTP redirects without executing the final request.
func resolveRedirects(shortURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", shortURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")

	currentURL := shortURL
	for i := 0; i < 10; i++ {
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("following redirect: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return currentURL, nil
			}
			// Handle relative redirects
			locURL, err := url.Parse(loc)
			if err != nil {
				return "", fmt.Errorf("parsing redirect location: %w", err)
			}
			base, _ := url.Parse(currentURL)
			resolved := base.ResolveReference(locURL)
			currentURL = resolved.String()

			req, err = http.NewRequest("GET", currentURL, nil)
			if err != nil {
				return "", fmt.Errorf("creating redirect request: %w", err)
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1")
			continue
		}

		return currentURL, nil
	}

	return currentURL, nil
}

// IsDouyinURL checks if the given string contains a Douyin URL.
func IsDouyinURL(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Contains(s, "douyin.com")
}

// ExtractURL extracts a Douyin URL from text that may contain other content.
func ExtractURL(text string) string {
	if match := douyinShortURLPattern.FindString(text); match != "" {
		return match
	}
	// Try to find any douyin.com URL
	re := regexp.MustCompile(`https?://[^\s]*douyin\.com[^\s]*`)
	if match := re.FindString(text); match != "" {
		return match
	}
	return ""
}
