// Package cache stores downloaded videos and transcripts under /tmp.
// The operating system deletes /tmp, and entries older than TTL are not reused.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const TTL = 24 * time.Hour

// Root is the cache directory. Tests replace it.
var Root = filepath.Join(os.TempDir(), "dyt")

// Record is one finished transcript.
type Record struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	URL    string    `json:"url"`
	Input  string    `json:"input"`
	Text   string    `json:"text"`
	Speech string    `json:"speech"`
	Chat   string    `json:"chat"`
	At     time.Time `json:"at"`
}

// Dir is the directory for one video.
func Dir(id string) string {
	return filepath.Join(Root, id)
}

// Fresh reports whether path exists, is non-empty, and is inside the TTL.
func Fresh(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return false
	}
	return time.Since(info.ModTime()) < TTL
}

// Save writes a transcript next to the cached video.
func Save(rec Record) error {
	if rec.At.IsZero() {
		rec.At = time.Now()
	}
	dir := Dir(rec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fileName(rec.Speech, rec.Chat)+".json"), append(raw, '\n'), 0o644)
}

// Load returns the transcript for this video and model pair.
func Load(id, speech, chat string) (Record, bool) {
	path := filepath.Join(Dir(id), fileName(speech, chat)+".json")
	if !Fresh(path) {
		return Record{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false
	}
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Record{}, false
	}
	return rec, true
}

// Remove deletes one video's cached file and transcripts.
func Remove(id string) error {
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid cache id %q", id)
	}
	err := os.RemoveAll(Dir(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns the newest transcript of each cached video, newest first.
// Entries older than TTL are left for the system to delete.
func List() ([]Record, error) {
	entries, err := os.ReadDir(Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Record
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(Root, entry.Name(), "*.json"))
		if err != nil {
			continue
		}
		var newest Record
		var found bool
		for _, file := range files {
			if !Fresh(file) {
				continue
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			var rec Record
			if err := json.Unmarshal(raw, &rec); err != nil || rec.Text == "" {
				continue
			}
			if !found || rec.At.After(newest.At) {
				newest = rec
				found = true
			}
		}
		if found {
			out = append(out, newest)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

func fileName(speech, chat string) string {
	sum := sha256.Sum256([]byte(speech + "\n" + chat))
	return hex.EncodeToString(sum[:8])
}
