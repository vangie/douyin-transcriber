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
)

// routerDataPattern matches window._ROUTER_DATA in the HTML.
var routerDataPattern = regexp.MustCompile(`window\._ROUTER_DATA\s*=\s*(.*?)</script>`)

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

// FetchVideoInfo gets video metadata from Douyin's mobile web page.
// This approach does not require cookies, login, or browser signatures.
func FetchVideoInfo(videoID string) (*VideoInfo, error) {
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
