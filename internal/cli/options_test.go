package cli

import (
	"testing"
	"time"
)

func TestParseOptionsReadsFlagsAndPrompt(t *testing.T) {
	opts, err := ParseOptions([]string{"--json", "--model", "gpt-5.4", "--cwd", "/tmp/demo", "--timeout", "45s", "draw a dragon"})
	if err != nil {
		t.Fatalf("ParseOptions returned error: %v", err)
	}

	if !opts.JSON {
		t.Fatal("expected JSON to be true")
	}
	if opts.Model != "gpt-5.4" {
		t.Fatalf("model = %q", opts.Model)
	}
	if opts.CWD != "/tmp/demo" {
		t.Fatalf("cwd = %q", opts.CWD)
	}
	if opts.Timeout != 45*time.Second {
		t.Fatalf("timeout = %v", opts.Timeout)
	}
	if opts.Prompt != "draw a dragon" {
		t.Fatalf("prompt = %q", opts.Prompt)
	}
}
