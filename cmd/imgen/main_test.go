package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/config"
)

func TestServeGeneratorUsesSchedulerGlobalMaxConcurrency(t *testing.T) {
	fake := newBlockingMainTestGenerator(2)
	generator := serveGenerator(fake, config.Config{Scheduler: config.SchedulerConfig{GlobalMaxConcurrency: 1}})

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = generator.Generate(context.Background(), backend.GenerateRequest{ImageIndex: 1})
	})
	waitForMainTestEntry(t, fake.entered)

	wg.Go(func() {
		_, _ = generator.Generate(context.Background(), backend.GenerateRequest{ImageIndex: 2})
	})
	assertNoMainTestEntry(t, fake.entered)

	close(fake.release)
	waitForMainTestWorkers(t, &wg)
}

type blockingMainTestGenerator struct {
	entered chan backend.GenerateRequest
	release chan struct{}
}

func newBlockingMainTestGenerator(enteredBuffer int) *blockingMainTestGenerator {
	return &blockingMainTestGenerator{
		entered: make(chan backend.GenerateRequest, enteredBuffer),
		release: make(chan struct{}),
	}
}

func (g *blockingMainTestGenerator) Generate(ctx context.Context, req backend.GenerateRequest) (backend.GenerateResult, error) {
	select {
	case g.entered <- req:
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
	select {
	case <-g.release:
		return backend.GenerateResult{Path: "ok.png"}, nil
	case <-ctx.Done():
		return backend.GenerateResult{}, ctx.Err()
	}
}

func waitForMainTestEntry(t *testing.T, entered <-chan backend.GenerateRequest) backend.GenerateRequest {
	t.Helper()
	select {
	case req := <-entered:
		return req
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for backend Generate call")
		return backend.GenerateRequest{}
	}
}

func assertNoMainTestEntry(t *testing.T, entered <-chan backend.GenerateRequest) {
	t.Helper()
	select {
	case req := <-entered:
		t.Fatalf("unexpected backend Generate call for image index %d", req.ImageIndex)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitForMainTestWorkers(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Generate calls did not finish")
	}
}
