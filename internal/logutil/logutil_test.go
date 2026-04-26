package logutil

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestStamp(t *testing.T) {
	now := time.Date(2026, 4, 25, 23, 22, 16, 0, time.FixedZone("CST", 8*60*60))
	got := Stamp(now, "image attempt started job_id=job_1 image_index=1 attempt=1")
	want := "[2026-04-25T23:22:16+08:00] image attempt started job_id=job_1 image_index=1 attempt=1"
	if got != want {
		t.Fatalf("Stamp() = %q, want %q", got, want)
	}
}

func TestPrintlnWritesTimestampedMessageToStdout(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		Println("image attempt started job_id=job_1")
	})

	assertTimestampedOutput(t, stdout, "image attempt started job_id=job_1")
	assertEmptyOutput(t, stderr, "stderr")
}

func TestPrintfWritesTimestampedFormattedMessageToStdout(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		Printf("image attempt started job_id=%s attempt=%d", "job_1", 2)
	})

	assertTimestampedOutput(t, stdout, "image attempt started job_id=job_1 attempt=2")
	assertEmptyOutput(t, stderr, "stderr")
}

func TestWarnfWritesTimestampedFormattedMessageToStderr(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		Warnf("image attempt failed job_id=%s error=%q", "job_1", "temporary failure")
	})

	assertEmptyOutput(t, stdout, "stdout")
	assertTimestampedOutput(t, stderr, "image attempt failed job_id=job_1 error=\"temporary failure\"")
}

func TestErrorfWritesTimestampedFormattedMessageToStderr(t *testing.T) {
	stdout, stderr := captureStreams(t, func() {
		Errorf("maintenance notification failed job_id=%s error=%v", "job_1", "smtp unavailable")
	})

	assertEmptyOutput(t, stdout, "stdout")
	assertTimestampedOutput(t, stderr, "maintenance notification failed job_id=job_1 error=smtp unavailable")
}

func captureStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout returned error: %v", err)
	}
	defer stdoutReader.Close()
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutWriter.Close()
		t.Fatalf("os.Pipe stderr returned error: %v", err)
	}
	defer stderrReader.Close()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	fn()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("closing stdout writer returned error: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("closing stderr writer returned error: %v", err)
	}

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("reading stdout returned error: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("reading stderr returned error: %v", err)
	}
	return string(stdout), string(stderr)
}

func assertTimestampedOutput(t *testing.T, got string, wantMessage string) {
	t.Helper()

	if !strings.HasSuffix(got, wantMessage+"\n") {
		t.Fatalf("output = %q, want suffix %q", got, wantMessage+"\n")
	}

	matched, err := regexp.MatchString(`^\[\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})\] `, got)
	if err != nil {
		t.Fatalf("regexp.MatchString returned error: %v", err)
	}
	if !matched {
		t.Fatalf("output = %q, want RFC3339 timestamp prefix in square brackets", got)
	}
}

func assertEmptyOutput(t *testing.T, got string, stream string) {
	t.Helper()
	if got != "" {
		t.Fatalf("%s = %q, want empty", stream, got)
	}
}
