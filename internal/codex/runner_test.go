package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunnerRunReturnsDeadlineExceededOnTimeout(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "sleep 0.2"},
		Timeout: 10 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRunnerRunDoesNotStartCommandWhenContextCanceled(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var phases []string
	_, err := (Runner{}).Run(ctx, Request{
		Command: "sh",
		Args:    []string{"-c", "printf started > \"$SENTINEL\""},
		Env:     []string{"SENTINEL=" + sentinel, "PATH=/bin:/usr/bin"},
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if len(phases) != 0 {
		t.Fatalf("expected no process phases because command was not started, got %#v", phases)
	}
	if _, statErr := os.Stat(sentinel); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("sentinel file exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestRunnerRunTimeoutKillsProcessGroupBeforeWaitingForPipes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group timeout behavior is Unix-specific")
	}
	started := time.Now()
	_, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "sh -c 'sleep 2' & printf 'parent ready\\n'; wait"},
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(started)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Run returned after %s, want it to return promptly after timeout", elapsed)
	}
}

func TestRunnerRunRecordsStreamingPhases(t *testing.T) {
	var phases []string
	result, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "printf '{\"type\":\"thread.started\",\"thread_id\":\"thread_123\"}\n'; printf '{\"type\":\"turn.started\"}\n'; printf 'Saved to: file:///tmp/out.png\n'"},
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.Stdout, "thread.started") || !strings.Contains(result.Stdout, "Saved to:") {
		t.Fatalf("stdout = %q", result.Stdout)
	}

	want := []string{"process.starting", "process.started", "stdout.first_line", "stdout.thread_started", "stdout.turn_started", "stdout.saved_to", "process.exited"}
	if !reflect.DeepEqual(phases, want) {
		t.Fatalf("phases = %#v, want %#v", phases, want)
	}
}

func TestRunnerRunRecordsTurnCompletedUsage(t *testing.T) {
	var phases []string
	var details []string
	result, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "printf '{\"type\":\"thread.started\",\"thread_id\":\"thread_123\"}\n'; printf '{\"type\":\"turn.started\"}\n'; printf '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":21514,\"cached_input_tokens\":2432,\"output_tokens\":541,\"reasoning_output_tokens\":115}}\n'"},
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
			if phase == "stdout.turn_completed" {
				details = append(details, detail)
			}
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.Stdout, "turn.completed") {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if !phaseBefore(phases, "stdout.turn_started", "stdout.turn_completed") {
		t.Fatalf("expected stdout.turn_started before stdout.turn_completed, phases=%#v", phases)
	}
	if !phaseBefore(phases, "stdout.turn_completed", "process.exited") {
		t.Fatalf("expected stdout.turn_completed before process.exited, phases=%#v", phases)
	}
	wantDetails := []string{"input_tokens=21514 cached_input_tokens=2432 output_tokens=541 reasoning_output_tokens=115"}
	if !reflect.DeepEqual(details, wantDetails) {
		t.Fatalf("details = %#v, want %#v", details, wantDetails)
	}
}

func TestRunnerRunRecordsStderrFirstLine(t *testing.T) {
	var phases []string
	result, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "printf 'warning from codex\n' >&2"},
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.Stderr, "warning from codex") {
		t.Fatalf("stderr = %q", result.Stderr)
	}

	want := []string{"process.starting", "process.started", "stderr.first_line", "process.exited"}
	if !reflect.DeepEqual(phases, want) {
		t.Fatalf("phases = %#v, want %#v", phases, want)
	}
}

func TestRunnerRunRecordsImageFileDetected(t *testing.T) {
	codexHome := t.TempDir()
	threadID := "thread_file_detected"
	imageDir := filepath.Join(codexHome, "generated_images", threadID)
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	markerPath := filepath.Join(codexHome, "detected.marker")

	var phases []string
	var imageDetectedDetails []string
	result, err := (Runner{}).Run(context.Background(), Request{
		Command:   "sh",
		Args:      []string{"-c", "printf '{\"type\":\"thread.started\",\"thread_id\":\"thread_file_detected\"}\n'; printf '{\"type\":\"turn.started\"}\n'; i=0; while [ ! -f \"$MARKER\" ] && [ $i -lt 100 ]; do i=$((i+1)); sleep 0.02; done"},
		Env:       []string{"MARKER=" + markerPath, "PATH=/bin:/usr/bin"},
		CodexHome: codexHome,
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
			if phase == "stdout.thread_started" {
				if err := os.WriteFile(filepath.Join(imageDir, "ig_test.png"), []byte("png"), 0o644); err != nil {
					t.Errorf("WriteFile returned error: %v", err)
				}
			}
			if phase == "image.file_detected" {
				imageDetectedDetails = append(imageDetectedDetails, detail)
				if err := os.WriteFile(markerPath, []byte("detected"), 0o644); err != nil {
					t.Errorf("WriteFile marker returned error: %v", err)
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.Stdout, "thread.started") {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if !phaseBefore(phases, "image.file_detected", "process.exited") {
		t.Fatalf("expected image.file_detected before process.exited, phases=%#v", phases)
	}
	if !reflect.DeepEqual(imageDetectedDetails, []string{"ext=.png"}) {
		t.Fatalf("image.file_detected details = %#v, want %#v", imageDetectedDetails, []string{"ext=.png"})
	}
}

func TestRunnerRunStartsImageFileDetectorAfterFirstNonEmptyThreadID(t *testing.T) {
	codexHome := t.TempDir()
	threadID := "thread_late_valid"
	imageDir := filepath.Join(codexHome, "generated_images", threadID)
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	markerPath := filepath.Join(codexHome, "detected.marker")

	var phases []string
	_, err := (Runner{}).Run(context.Background(), Request{
		Command:   "sh",
		Args:      []string{"-c", "printf '{\"type\":\"thread.started\",\"thread_id\":\"\"}\n'; printf '{\"type\":\"turn.started\"}\n'; printf '{\"type\":\"thread.started\",\"thread_id\":\"thread_late_valid\"}\n'; i=0; while [ ! -f \"$MARKER\" ] && [ $i -lt 100 ]; do i=$((i+1)); sleep 0.02; done"},
		Env:       []string{"MARKER=" + markerPath, "PATH=/bin:/usr/bin"},
		CodexHome: codexHome,
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
			if phase == "stdout.thread_started" {
				if err := os.WriteFile(filepath.Join(imageDir, "ig_test.jpg"), []byte("jpg"), 0o644); err != nil {
					t.Errorf("WriteFile returned error: %v", err)
				}
			}
			if phase == "image.file_detected" {
				if err := os.WriteFile(markerPath, []byte("detected"), 0o644); err != nil {
					t.Errorf("WriteFile marker returned error: %v", err)
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !phaseBefore(phases, "image.file_detected", "process.exited") {
		t.Fatalf("expected image.file_detected before process.exited, phases=%#v", phases)
	}
}

func TestRunnerRunSkipsImageFileDetectedWithoutCodexHome(t *testing.T) {
	var phases []string
	_, err := (Runner{}).Run(context.Background(), Request{
		Command: "sh",
		Args:    []string{"-c", "printf '{\"type\":\"thread.started\",\"thread_id\":\"thread_123\"}\n'; printf '{\"type\":\"turn.started\"}\n'"},
		RecordPhase: func(phase string, occurredAt time.Time, detail string) {
			phases = append(phases, phase)
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, phase := range phases {
		if phase == "image.file_detected" {
			t.Fatalf("did not expect image.file_detected, phases=%#v", phases)
		}
	}
}

func TestImageFileDetectorCleanupWaitsForInFlightRecord(t *testing.T) {
	codexHome := t.TempDir()
	threadID := "thread_cleanup_waits"
	imageDir := filepath.Join(codexHome, "generated_images", threadID)
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "ig_test.webp"), []byte("webp"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	recordEntered := make(chan struct{})
	releaseRecord := make(chan struct{})
	cleanup := startImageFileDetector(context.Background(), codexHome, threadID, func(phase string, detail string) {
		if phase != "image.file_detected" || detail != "ext=.webp" {
			t.Errorf("recorded phase=%q detail=%q", phase, detail)
		}
		close(recordEntered)
		<-releaseRecord
	})

	select {
	case <-recordEntered:
	case <-time.After(time.Second):
		t.Fatal("detector did not start image.file_detected record")
	}

	cleanupDone := make(chan struct{})
	go func() {
		cleanup()
		close(cleanupDone)
	}()

	select {
	case <-cleanupDone:
		t.Fatal("cleanup returned before in-flight record completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseRecord)
	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not return after in-flight record completed")
	}
}

func TestRunnerRunOverridesInheritedCodexHome(t *testing.T) {
	codexHome := t.TempDir()
	result, err := (Runner{}).Run(context.Background(), Request{
		Command:   "sh",
		Args:      []string{"-c", "printf '%s' \"$CODEX_HOME\""},
		Env:       []string{"CODEX_HOME=/tmp/openclaw-isolated", "PATH=/bin:/usr/bin"},
		CodexHome: codexHome,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != codexHome {
		t.Fatalf("CODEX_HOME = %q, want %q", result.Stdout, codexHome)
	}
}

func phaseBefore(phases []string, before string, after string) bool {
	beforeIndex := -1
	afterIndex := -1
	for i, phase := range phases {
		if phase == before && beforeIndex == -1 {
			beforeIndex = i
		}
		if phase == after && afterIndex == -1 {
			afterIndex = i
		}
	}
	return beforeIndex >= 0 && afterIndex >= 0 && beforeIndex < afterIndex
}
