package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PhaseRecorder func(phase string, occurredAt time.Time, detail string)

type Request struct {
	Command     string
	Args        []string
	Dir         string
	Env         []string
	Timeout     time.Duration
	CodexHome   string
	RecordPhase PhaseRecorder
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner struct{}

func (Runner) Run(ctx context.Context, req Request) (RunResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	var recordMu sync.Mutex
	record := func(phase string, detail string) {
		if req.RecordPhase == nil {
			return
		}
		recordMu.Lock()
		defer recordMu.Unlock()
		req.RecordPhase(phase, time.Now(), detail)
	}

	record("process.starting", "")
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = envWithCodexHome(req.Env, req.CodexHome)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return RunResult{}, err
	}

	if err := cmd.Start(); err != nil {
		record("process.start_failed", "error_len="+strconv.Itoa(len(err.Error())))
		return RunResult{}, err
	}
	record("process.started", "")

	var cleanupMu sync.Mutex
	var cleanups []context.CancelFunc
	addCleanup := func(cancel context.CancelFunc) {
		cleanupMu.Lock()
		defer cleanupMu.Unlock()
		cleanups = append(cleanups, cancel)
	}
	runCleanups := func() {
		cleanupMu.Lock()
		defer cleanupMu.Unlock()
		for _, cancel := range cleanups {
			cancel()
		}
		cleanups = nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var wg sync.WaitGroup
	var stdoutErr error
	var stderrErr error
	wg.Go(func() {
		stdoutErr = readLines(stdoutPipe, &stdout, stdoutPhaseRecorder(ctx, req.CodexHome, record, addCleanup))
	})
	wg.Go(func() {
		stderrErr = readLines(stderrPipe, &stderr, stderrPhaseRecorder(record))
	})

	wg.Wait()
	waitErr := cmd.Wait()
	runCleanups()
	record("process.exited", "")

	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if stdoutErr != nil && !errors.Is(stdoutErr, io.EOF) {
		waitErr = stdoutErr
	}
	if stderrErr != nil && !errors.Is(stderrErr, io.EOF) {
		waitErr = stderrErr
	}
	if waitErr == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}
	return result, waitErr
}

func envWithCodexHome(env []string, codexHome string) []string {
	if codexHome == "" {
		return env
	}
	filtered := make([]string, 0, len(env)+1)
	for _, value := range env {
		if !strings.HasPrefix(value, "CODEX_HOME=") {
			filtered = append(filtered, value)
		}
	}
	return append(filtered, "CODEX_HOME="+codexHome)
}

func readLines(reader io.Reader, output *bytes.Buffer, onLine func(string)) error {
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadString('\n')
		if line != "" {
			output.WriteString(line)
			onLine(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func startImageFileDetector(ctx context.Context, codexHome string, threadID string, record func(string, string)) context.CancelFunc {
	detectorCtx, cancel := context.WithCancel(ctx)
	if codexHome == "" || !isSinglePathSegment(threadID) {
		return cancel
	}
	dir := filepath.Join(codexHome, "generated_images", threadID)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-detectorCtx.Done():
				return
			default:
			}
			if ext, ok := detectImageFile(dir); ok {
				select {
				case <-detectorCtx.Done():
					return
				default:
					record("image.file_detected", "ext="+ext)
					return
				}
			}
			select {
			case <-detectorCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func isSinglePathSegment(value string) bool {
	return value != "" && value == filepath.Base(value) && value != "." && value != ".."
}

func detectImageFile(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
			return ext, true
		}
	}
	return "", false
}

func stdoutPhaseRecorder(ctx context.Context, codexHome string, record func(string, string), addCleanup func(context.CancelFunc)) func(string) {
	seenFirst := false
	seenThread := false
	seenTurn := false
	seenTurnCompleted := false
	seenSavedTo := false
	startedDetector := false
	return func(line string) {
		if !seenFirst {
			seenFirst = true
			record("stdout.first_line", "line_len="+strconv.Itoa(len(line)))
		}
		trimmed := strings.TrimSpace(line)
		if !seenThread || !seenTurn || !seenTurnCompleted || !startedDetector {
			var event struct {
				Type     string `json:"type"`
				ThreadID string `json:"thread_id"`
				Usage    struct {
					InputTokens           int `json:"input_tokens"`
					CachedInputTokens     int `json:"cached_input_tokens"`
					OutputTokens          int `json:"output_tokens"`
					ReasoningOutputTokens int `json:"reasoning_output_tokens"`
				} `json:"usage"`
			}
			if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &event) == nil {
				if event.Type == "thread.started" {
					if !seenThread {
						seenThread = true
						record("stdout.thread_started", "thread_id_len="+strconv.Itoa(len(event.ThreadID)))
					}
					if !startedDetector && codexHome != "" && isSinglePathSegment(event.ThreadID) {
						startedDetector = true
						addCleanup(startImageFileDetector(ctx, codexHome, event.ThreadID, record))
					}
				}
				if event.Type == "turn.started" && !seenTurn {
					seenTurn = true
					record("stdout.turn_started", "")
				}
				if event.Type == "turn.completed" && !seenTurnCompleted {
					seenTurnCompleted = true
					record("stdout.turn_completed", turnCompletedDetail(event.Usage.InputTokens, event.Usage.CachedInputTokens, event.Usage.OutputTokens, event.Usage.ReasoningOutputTokens))
				}
			}
		}
		if !seenSavedTo && (strings.HasPrefix(trimmed, "Saved to:") || strings.HasPrefix(trimmed, "file://")) {
			seenSavedTo = true
			record("stdout.saved_to", "line_len="+strconv.Itoa(len(line)))
		}
	}
}

func turnCompletedDetail(inputTokens int, cachedInputTokens int, outputTokens int, reasoningOutputTokens int) string {
	return "input_tokens=" + strconv.Itoa(inputTokens) +
		" cached_input_tokens=" + strconv.Itoa(cachedInputTokens) +
		" output_tokens=" + strconv.Itoa(outputTokens) +
		" reasoning_output_tokens=" + strconv.Itoa(reasoningOutputTokens)
}

func stderrPhaseRecorder(record func(string, string)) func(string) {
	seenFirst := false
	return func(line string) {
		if seenFirst {
			return
		}
		seenFirst = true
		record("stderr.first_line", "line_len="+strconv.Itoa(len(line)))
	}
}
