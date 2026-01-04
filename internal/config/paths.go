package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveDBPath finds the Beeper index.db path based on flags, env, or defaults.
func ResolveDBPath(explicit string) (string, error) {
	tried := []string{}

	if explicit != "" {
		path := expandPath(explicit)
		if fileExists(path) {
			return path, nil
		}
		return "", fmt.Errorf("database not found at %s", path)
	}

	if env := os.Getenv("BEEPER_DB"); env != "" {
		path := expandPath(env)
		tried = append(tried, path)
		if fileExists(path) {
			return path, nil
		}
	}

	for _, path := range defaultPaths() {
		path = expandPath(path)
		tried = append(tried, path)
		if fileExists(path) {
			return path, nil
		}
	}

	for _, path := range globCandidates() {
		tried = append(tried, path)
		if fileExists(path) {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not find Beeper database; tried: %s", strings.Join(tried, ", "))
}

func defaultPaths() []string {
	var paths []string
	paths = append(paths, []string{
		"~/Library/Application Support/BeeperTexts/index.db",
		"~/Library/Application Support/Beeper/index.db",
	}...)

	switch runtime.GOOS {
	case "linux":
		paths = append(paths,
			"~/.config/BeeperTexts/index.db",
			"~/.config/Beeper/index.db",
		)
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			paths = append(paths,
				filepath.Join(appData, "BeeperTexts", "index.db"),
				filepath.Join(appData, "Beeper", "index.db"),
			)
		}
	}

	return paths
}

func globCandidates() []string {
	pattern := expandPath("~/Library/Application Support/Beeper*/**/index.db")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return matches
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return os.ExpandEnv(path)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return err == nil && !info.IsDir()
}
