package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/parser"
)

type CommandRunner interface {
	Run(context.Context, codex.Request) (codex.RunResult, error)
}

type BuiltinCodex struct {
	Command          string
	Model            string
	CWD              string
	Timeout          time.Duration
	DeliveryDir      string
	DeliveryMaxFiles int
	CodexHome        string
	Runner           CommandRunner
}

func (b BuiltinCodex) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	args := []string{"exec", "--json"}
	model := req.Model
	if model == "" {
		model = b.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	cwd := req.CWD
	if cwd == "" {
		cwd = b.CWD
	}
	if cwd != "" {
		args = append(args, "--cd", cwd)
	}
	for _, image := range req.Images {
		args = append(args, "--image", image)
	}
	args = append(args, "--", req.Prompt)

	runner := b.Runner
	if runner == nil {
		runner = codex.Runner{}
	}
	result, err := runner.Run(ctx, codex.Request{
		Command:   b.command(),
		Args:      args,
		Env:       os.Environ(),
		Timeout:   b.timeout(),
		CodexHome: b.codexHome(),
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			if req.RecordPhase != nil {
				req.RecordPhase(phase, occurredAt, detail)
			}
		},
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("codex exec failed: %s", formatCommandError(err, result))
	}
	if req.RecordPhase != nil {
		req.RecordPhase("parser.started", time.Now(), "")
	}
	parsed, err := parser.ExtractImageResult(result.Stdout, b.codexHome())
	if err != nil {
		if req.RecordPhase != nil {
			req.RecordPhase("parser.failed", time.Now(), "error_len="+strconv.Itoa(len(err.Error())))
		}
		return GenerateResult{}, err
	}
	if req.RecordPhase != nil {
		req.RecordPhase("parser.completed", time.Now(), "path_len="+strconv.Itoa(len(parsed.Path)))
	}
	path := parsed.Path
	uri := parsed.URI
	if b.DeliveryDir != "" {
		path, err = copyImageToDeliveryDir(parsed.Path, b.DeliveryDir)
		if err != nil {
			return GenerateResult{}, err
		}
		if err := pruneDeliveryDir(b.DeliveryDir, b.DeliveryMaxFiles); err != nil {
			return GenerateResult{}, err
		}
		uri = (&url.URL{Scheme: "file", Path: path}).String()
	}
	return GenerateResult{Path: path, URI: uri, RawOutput: result.Stdout + result.Stderr}, nil
}

func copyImageToDeliveryDir(sourcePath string, deliveryDir string) (string, error) {
	if err := os.MkdirAll(deliveryDir, 0o755); err != nil {
		return "", err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(deliveryDir, deliveryFileName(sourcePath))
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func deliveryFileName(sourcePath string) string {
	cleanPath := filepath.Clean(sourcePath)
	base := filepath.Base(cleanPath)
	parent := filepath.Base(filepath.Dir(cleanPath))
	sum := sha256.Sum256([]byte(cleanPath))
	hash := hex.EncodeToString(sum[:4])
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		return hash + "-" + base
	}
	return parent + "-" + hash + "-" + base
}

func pruneDeliveryDir(deliveryDir string, maxFiles int) error {
	if maxFiles <= 0 {
		return nil
	}
	entries, err := os.ReadDir(deliveryDir)
	if err != nil {
		return err
	}
	type deliveryFile struct {
		path    string
		name    string
		modTime time.Time
	}
	files := make([]deliveryFile, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, deliveryFile{
			path:    filepath.Join(deliveryDir, entry.Name()),
			name:    entry.Name(),
			modTime: info.ModTime(),
		})
	}
	if len(files) <= maxFiles {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].name > files[j].name
		}
		return files[i].modTime.After(files[j].modTime)
	})
	for _, file := range files[maxFiles:] {
		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (b BuiltinCodex) command() string {
	if b.Command != "" {
		return b.Command
	}
	return "codex"
}

func (b BuiltinCodex) codexHome() string {
	if b.CodexHome != "" {
		return b.CodexHome
	}
	home, _ := os.UserHomeDir()
	return home + "/.codex"
}

func (b BuiltinCodex) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return 90 * time.Second
}

func formatCommandError(err error, result codex.RunResult) string {
	parts := []string{"execution failed"}
	if err == context.DeadlineExceeded {
		parts = append(parts, "deadline exceeded")
	} else if err != nil {
		parts = append(parts, err.Error())
	}
	if result.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit_code=%d", result.ExitCode))
	}
	stderr := summarizeOutput(result.Stderr)
	if stderr != "" {
		parts = append(parts, fmt.Sprintf("stderr: %s", stderr))
	}
	stdout := summarizeOutput(result.Stdout)
	if stdout != "" {
		parts = append(parts, fmt.Sprintf("stdout: %s", stdout))
	}
	return strings.Join(parts, "; ")
}

func summarizeOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.ReplaceAll(output, "\n", " | ")
	if len(output) <= 200 {
		return output
	}
	return output[:200] + "..."
}
