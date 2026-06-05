package cli

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/backend"
)

type recordingGenerator struct {
	mu      sync.Mutex
	prompts []string
	images  [][]string
	active  int
	maxSeen int
	block   chan struct{}
}

type flakyGenerator struct {
	mu       sync.Mutex
	attempts []int
	failures int
}

func (g *recordingGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	g.prompts = append(g.prompts, req.Prompt)
	g.images = append(g.images, append([]string(nil), req.Images...))
	g.active++
	if g.active > g.maxSeen {
		g.maxSeen = g.active
	}
	g.mu.Unlock()

	if g.block != nil {
		<-g.block
	}

	g.mu.Lock()
	g.active--
	idx := len(g.prompts)
	g.mu.Unlock()
	return backend.GenerateResult{Path: "/tmp/" + string(rune('0'+idx)) + ".png"}, nil
}

func (g *flakyGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempts = append(g.attempts, req.Attempt)
	if len(g.attempts) <= g.failures {
		return backend.GenerateResult{}, errors.New("TooManyRequests")
	}
	return backend.GenerateResult{Path: "/tmp/retry-success.png"}, nil
}

func TestLocalEngineBuildsPromptForEachImage(t *testing.T) {
	gen := &recordingGenerator{}
	engine := LocalEngine{Generator: gen, Prefix: "$imagegen", Prelude: "使用内置 imagegen 技能。"}

	res, err := engine.RunSync(context.Background(), SyncRequest{Prompt: "draw a dragon", Count: 2, Concurrency: 1})
	if err != nil {
		t.Fatalf("RunSync returned error: %v", err)
	}
	if len(res.Images) != 2 {
		t.Fatalf("images = %d", len(res.Images))
	}
	if len(gen.prompts) != 2 {
		t.Fatalf("prompts = %d", len(gen.prompts))
	}
	want := "使用内置 imagegen 技能。\n\n$imagegen draw a dragon"
	if gen.prompts[0] != want {
		t.Fatalf("prompt = %q, want %q", gen.prompts[0], want)
	}
}

func TestLocalEngineRespectsConcurrencyLimit(t *testing.T) {
	gen := &recordingGenerator{block: make(chan struct{})}
	engine := LocalEngine{Generator: gen, Prefix: "$imagegen"}

	done := make(chan struct{})
	go func() {
		_, _ = engine.RunSync(context.Background(), SyncRequest{Prompt: "draw a dragon", Count: 3, Concurrency: 2})
		close(done)
	}()

	for {
		gen.mu.Lock()
		maxSeen := gen.maxSeen
		calls := len(gen.prompts)
		gen.mu.Unlock()
		if calls >= 2 {
			if maxSeen != 2 {
				t.Fatalf("maxSeen = %d, want 2", maxSeen)
			}
			close(gen.block)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	<-done
}

func TestLocalEnginePassesImagesToGenerator(t *testing.T) {
	gen := &recordingGenerator{}
	engine := LocalEngine{Generator: gen, Prefix: "$imagegen"}
	_, err := engine.RunSync(context.Background(), SyncRequest{Prompt: "draw a dragon", Images: []string{"/tmp/1.png"}, Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("RunSync returned error: %v", err)
	}
	if len(gen.images) != 1 || len(gen.images[0]) != 1 || gen.images[0][0] != "/tmp/1.png" {
		t.Fatalf("images = %v", gen.images)
	}
}

func TestLocalEngineRetriesSyncGenerationUpToMaxAttempts(t *testing.T) {
	gen := &flakyGenerator{failures: 2}
	engine := LocalEngine{Generator: gen, Prefix: "$imagegen", MaxAttempts: 3, RetryDelays: []time.Duration{0}}

	res, err := engine.RunSync(context.Background(), SyncRequest{Prompt: "draw a dragon", Count: 1, Concurrency: 1})
	if err != nil {
		t.Fatalf("RunSync returned error: %v", err)
	}
	if len(res.Images) != 1 || res.Images[0].Path != "/tmp/retry-success.png" {
		t.Fatalf("images = %+v", res.Images)
	}
	wantAttempts := []int{1, 2, 3}
	if len(gen.attempts) != len(wantAttempts) {
		t.Fatalf("attempts = %#v, want %#v", gen.attempts, wantAttempts)
	}
	for i, want := range wantAttempts {
		if gen.attempts[i] != want {
			t.Fatalf("attempts = %#v, want %#v", gen.attempts, wantAttempts)
		}
	}
}
