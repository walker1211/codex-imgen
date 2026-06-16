package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	api "github.com/walker1211/codex-imgen/internal/api"
)

type Client struct {
	BaseURL       string
	ImageInputDir string
}

func (c Client) CreateJob(ctx context.Context, prompt string, images []string, count int, concurrency int) (api.CreateJobResult, error) {
	stagedImages, err := c.stageImages(images)
	if err != nil {
		return api.CreateJobResult{}, err
	}
	return api.Client{BaseURL: c.BaseURL}.CreateJob(ctx, api.CreateJobRequest{
		Prompt:      prompt,
		Images:      stagedImages,
		Count:       count,
		Concurrency: concurrency,
	})
}

func (c Client) stageImages(images []string) ([]string, error) {
	if strings.TrimSpace(c.ImageInputDir) == "" {
		return images, nil
	}
	staged := make([]string, 0, len(images))
	for _, image := range images {
		trimmed := strings.TrimSpace(image)
		if trimmed == "" {
			continue
		}
		name, err := c.stageImage(trimmed)
		if err != nil {
			return nil, err
		}
		staged = append(staged, name)
	}
	return staged, nil
}

func (c Client) stageImage(image string) (string, error) {
	source, err := filepath.Abs(image)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("image path not found")
		}
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("image path must be a regular file")
	}
	if err := os.MkdirAll(c.ImageInputDir, 0o700); err != nil {
		return "", err
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	name := stagedImageName(source)
	if err := os.WriteFile(filepath.Join(c.ImageInputDir, name), content, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

func stagedImageName(source string) string {
	cleanPath := filepath.Clean(source)
	base := filepath.Base(cleanPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	sum := sha256.Sum256([]byte(cleanPath))
	hash := hex.EncodeToString(sum[:6])
	if stem == "" {
		return hash + ext
	}
	return stem + "-" + hash + ext
}

func (c Client) GetJob(ctx context.Context, jobID string) (api.JobStatus, error) {
	return api.Client{BaseURL: c.BaseURL}.GetJob(ctx, jobID)
}

func (c Client) ListJobs(ctx context.Context, limit int) ([]api.JobSummary, error) {
	return api.Client{BaseURL: c.BaseURL}.ListJobs(ctx, limit)
}

func (c Client) CancelJob(ctx context.Context, jobID string) error {
	return api.Client{BaseURL: c.BaseURL}.CancelJob(ctx, jobID)
}
