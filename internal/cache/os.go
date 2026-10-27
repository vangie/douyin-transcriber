package cache

import (
	"os"
	"time"
)

func chtimes(path string, at time.Time) error {
	return os.Chtimes(path, at, at)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
