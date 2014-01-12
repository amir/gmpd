package util

import (
	"os"
	"path/filepath"
	"runtime"
)

func HomeDir() string {
	return os.Getenv("HOME")
}

func CacheDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(HomeDir(), "Library", "Caches", "gmpd")
	}

	if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		return filepath.Join(xdgCacheHome, "gmpd")
	}

	return filepath.Join(HomeDir(), ".cache", "gmpd")
}
