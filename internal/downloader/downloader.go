// Package downloader handles Douyin video audio extraction.
// It uses Douyin's mobile web page to extract CDN URLs directly,
// without requiring yt-dlp, cookies, or browser signatures.
package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	mobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/121.0.2277.107 Version/17.0 Mobile/15E148 Safari/604.1"
	// The share page used to embed play URLs in window._ROUTER_DATA.videoInfoRes.
	// That field is gone; the mobile feed still returns the requested aweme
	// (mixed with others) and a playable mp4 that carries the audio track.
	feedURL = "https://aweme.snssdk.com/aweme/v1/feed/"
)

// routerDataPattern matches window._ROUTER_DATA in the HTML.
var routerDataPattern = regexp.MustCompile(`window\._ROUTER_DATA\s*=\s*(.*?)</script>`)

// feedBase overrides the feed endpoint in tests. Empty uses feedURL.
var feedBase string

// VideoInfo contains extracted video metadata and URLs.
type VideoInfo struct {
	Title    string
	VideoURL string
	AudioURL string
}

// DownloadAudio extracts audio from a Douyin video and saves it locally.
// videoURL should be a canonical URL: https://www.douyin.com/video/<ID>
// outputDir is optional; if empty, a temp directory is used.
func DownloadAudio(videoURL string, outputDir string) (string, error) {
	// Extract video ID from URL
	videoID, err := extractVideoIDFromURL(videoURL)
	if err != nil {
		return "", fmt.Errorf("extracting video ID: %w", err)
	}

	// Fetch video info from mobile page
	info, err := FetchVideoInfo(videoID)
	if err != nil {
		return "", fmt.Errorf("fetching video info: %w", err)
	}

	// Choose download URL: prefer audio-only, fall back to video
	downloadURL := info.AudioURL
	if downloadURL == "" {
		downloadURL = info.VideoURL
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no downloadable URL found for video %s", videoID)
	}

	fmt.Fprintf(os.Stderr, "Video: %s\n", oneLine(info.Title))

	// Create output directory
	if outputDir == "" {
		outputDir, err = os.MkdirTemp("", "dyt-*")
		if err != nil {
			return "", fmt.Errorf("creating temp dir: %w", err)
		}
	}

	// Download the media file
	mediaPath := filepath.Join(outputDir, videoID+".mp4")
	fmt.Fprintf(os.Stderr, "Downloading from CDN...\n")
	if err := downloadFile(downloadURL, mediaPath); err != nil {
		return "", fmt.Errorf("downloading media: %w", err)
	}

	// Extract audio with ffmpeg
	audioPath := filepath.Join(outputDir, videoID+".mp3")
	fmt.Fprintf(os.Stderr, "Extracting audio...\n")
	if err := extractAudio(mediaPath, audioPath); err != nil {
		return "", fmt.Errorf("extracting audio: %w", err)
	}

	// Clean up the video file
	os.Remove(mediaPath)

	return audioPath, nil
}

// FetchVideoInfo gets video metadata without cookies or a browser signature.
// The mobile feed is tried first. The old share-page parser remains as a
// fallback for responses that still embed videoInfoRes.
func FetchVideoInfo(videoID string) (*VideoInfo, error) {
	info, feedErr := fetchFromFeed(videoID)
	if feedErr == nil {
		return info, nil
	}
	info, pageErr := fetchFromSharePage(videoID)
	if pageErr == nil {
		return info, nil
	}
	return nil, fmt.Errorf("feed: %v; share page: %v", feedErr, pageErr)
}

func fetchFromFeed(videoID string) (*VideoInfo, error) {
	base := feedURL
	if feedBase != "" {
		base = feedBase
	}
	feed := fmt.Sprintf("%s?aweme_id=%s&version_code=270800&app_name=aweme", base, videoID)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", feed, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", mobileUA)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("reading feed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed status %d", resp.StatusCode)
	}

	var payload struct {
		StatusCode int `json:"status_code"`
		AwemeList  []struct {
			AwemeID string `json:"aweme_id"`
			Desc    string `json:"desc"`
			Video   struct {
				PlayAddr struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr"`
				PlayAddrH264 struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr_h264"`
			} `json:"video"`
		} `json:"aweme_list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parsing feed: %w", err)
	}
	if payload.StatusCode != 0 {
		return nil, fmt.Errorf("feed status_code %d", payload.StatusCode)
	}
	for _, item := range payload.AwemeList {
		if item.AwemeID != videoID {
			continue
		}
		// h264 files from this endpoint carry the aac track. play_addr may be
		// the same file or a video-only ladder, so prefer h264.
		videoURL := firstHTTP(item.Video.PlayAddrH264.URLList)
		if videoURL == "" {
			videoURL = firstHTTP(item.Video.PlayAddr.URLList)
		}
		if videoURL == "" {
			return nil, fmt.Errorf("aweme %s has no playable url", videoID)
		}
		return &VideoInfo{Title: item.Desc, VideoURL: videoURL}, nil
	}
	return nil, fmt.Errorf("aweme %s not in feed (%d items)", videoID, len(payload.AwemeList))
}

func firstHTTP(urls []string) string {
	for _, u := range urls {
		if strings.HasPrefix(u, "http") {
			return u
		}
	}
	return ""
}

func fetchFromSharePage(videoID string) (*VideoInfo, error) {
	shareURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s", videoID)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", shareURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", mobileUA)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching share page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	html := string(body)

	// Extract _ROUTER_DATA JSON
	matches := routerDataPattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("_ROUTER_DATA not found in page (status %d, length %d)", resp.StatusCode, len(html))
	}

	rawJSON := strings.TrimSpace(matches[1])

	// Parse the top-level JSON
	var routerData map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &routerData); err != nil {
		return nil, fmt.Errorf("parsing _ROUTER_DATA: %w", err)
	}

	// Navigate to loaderData -> "video_(id)/page" -> videoInfoRes -> item_list
	loaderRaw, ok := routerData["loaderData"]
	if !ok {
		return nil, fmt.Errorf("loaderData not found in _ROUTER_DATA")
	}

	var loaderData map[string]json.RawMessage
	if err := json.Unmarshal(loaderRaw, &loaderData); err != nil {
		return nil, fmt.Errorf("parsing loaderData: %w", err)
	}

	pageRaw, ok := loaderData["video_(id)/page"]
	if !ok {
		return nil, fmt.Errorf("video_(id)/page not found in loaderData")
	}

	var pageData map[string]json.RawMessage
	if err := json.Unmarshal(pageRaw, &pageData); err != nil {
		return nil, fmt.Errorf("parsing page data: %w", err)
	}

	videoInfoRaw, ok := pageData["videoInfoRes"]
	if !ok {
		return nil, fmt.Errorf("videoInfoRes not found in page data")
	}

	// Parse the video info structure
	var videoInfoRes struct {
		ItemList []struct {
			Desc  string `json:"desc"`
			Video struct {
				PlayAddr struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr"`
			} `json:"video"`
		} `json:"item_list"`
	}
	if err := json.Unmarshal(videoInfoRaw, &videoInfoRes); err != nil {
		return nil, fmt.Errorf("parsing videoInfoRes: %w", err)
	}

	if len(videoInfoRes.ItemList) == 0 {
		return nil, fmt.Errorf("no items in video info response")
	}

	item := videoInfoRes.ItemList[0]
	info := &VideoInfo{
		Title: item.Desc,
	}

	// Get video URL and convert to watermark-free version
	if len(item.Video.PlayAddr.URLList) > 0 {
		videoURL := item.Video.PlayAddr.URLList[0]
		// Replace "playwm" with "play" to get watermark-free URL
		info.VideoURL = strings.Replace(videoURL, "playwm", "play", 1)
	}

	return info, nil
}

// downloadFile downloads a URL to a local file.
func downloadFile(url, destPath string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", mobileUA)
	req.Header.Set("Referer", "https://www.douyin.com/")
	// The CDN gzips the mp4 when this is left at the Go default. ffmpeg then
	// sees a gzip stream instead of an mp4.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Downloaded %.1f MB\n", float64(written)/(1024*1024))
	return nil
}

// extractAudio extracts audio from a media file using ffmpeg.
func extractAudio(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath,
		"-vn", "-acodec", "libmp3lame", "-ab", "128k", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(out))
	}
	return nil
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}

func extractVideoIDFromURL(videoURL string) (string, error) {
	parts := strings.Split(strings.TrimRight(videoURL, "/"), "/")
	for i, p := range parts {
		if p == "video" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("cannot extract video ID from: %s", videoURL)
}

// Cleanup removes the temporary directory for a downloaded audio file.
func Cleanup(audioPath string) {
	dir := filepath.Dir(audioPath)
	base := filepath.Base(dir)
	// Only remove directories that match our naming convention
	if strings.HasPrefix(base, "dyt-") {
		os.RemoveAll(dir)
	}
}

// CheckDependencies verifies that ffmpeg is available.
// yt-dlp is no longer required.
func CheckDependencies() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: install with 'brew install ffmpeg'")
	}
	return nil
}
