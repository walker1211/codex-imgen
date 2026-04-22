package parser

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrImagePathNotFound = errors.New("image path not found in codex output")

type ImageResult struct {
	URI  string
	Path string
}

type threadStartedEvent struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
}

func ExtractImageResult(output string, codexHome string) (ImageResult, error) {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Saved to: file://") {
			uri := strings.TrimSpace(strings.TrimPrefix(trimmed, "Saved to:"))
			return imageResultFromURI(uri)
		}
		if trimmed == "Saved to:" && i+1 < len(lines) {
			uri := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(uri, "file://") {
				return imageResultFromURI(uri)
			}
		}
	}

	if codexHome != "" {
		if threadID := extractThreadID(lines); threadID != "" {
			return imageResultFromThreadDirectory(codexHome, threadID)
		}
	}

	return ImageResult{}, ErrImagePathNotFound
}

func extractThreadID(lines []string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
			continue
		}

		var event threadStartedEvent
		if err := json.Unmarshal([]byte(trimmed), &event); err != nil {
			continue
		}
		if event.Type == "thread.started" && event.ThreadID != "" {
			return event.ThreadID
		}
	}
	return ""
}

func imageResultFromThreadDirectory(codexHome string, threadID string) (ImageResult, error) {
	dir := filepath.Join(codexHome, "generated_images", threadID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ImageResult{}, ErrImagePathNotFound
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".png") || strings.HasSuffix(strings.ToLower(name), ".jpg") || strings.HasSuffix(strings.ToLower(name), ".jpeg") || strings.HasSuffix(strings.ToLower(name), ".webp") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ImageResult{}, ErrImagePathNotFound
	}

	sort.Strings(names)
	path := filepath.Join(dir, names[len(names)-1])
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return ImageResult{URI: uri, Path: path}, nil
}

func imageResultFromURI(uri string) (ImageResult, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return ImageResult{}, err
	}

	return ImageResult{
		URI:  uri,
		Path: parsed.Path,
	}, nil
}
