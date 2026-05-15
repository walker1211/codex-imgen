package backend

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestQueuedGeneratorLimitsConcurrentGenerateCalls(t *testing.T) {
	fake := newBlockingQueuedTestGenerator(2)
	generator := NewQueuedGenerator(fake, 1)

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = generator.Generate(context.Background(), GenerateRequest{ImageIndex: 1})
	})
	waitForQueuedTestEntry(t, fake.entered)

	wg.Go(func() {
		_, _ = generator.Generate(context.Background(), GenerateRequest{ImageIndex: 2})
	})
	assertNoQueuedTestEntry(t, fake.entered)

	close(fake.release)
	waitForQueuedTestWorkers(t, &wg)
}

func TestQueuedGeneratorReturnsContextErrorWhenCancelledWhileWaiting(t *testing.T) {
	fake := newBlockingQueuedTestGenerator(2)
	generator := NewQueuedGenerator(fake, 1)

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = generator.Generate(context.Background(), GenerateRequest{ImageIndex: 1})
	}()
	waitForQueuedTestEntry(t, fake.entered)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := generator.Generate(ctx, GenerateRequest{ImageIndex: 2})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	assertNoQueuedTestEntry(t, fake.entered)

	close(fake.release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first Generate call did not finish")
	}
}

func TestQueuedGeneratorReleasesSlotAfterBackendError(t *testing.T) {
	backendErr := errors.New("backend failed")
	fake := &scriptedQueuedTestGenerator{errors: []error{backendErr, nil}}
	generator := NewQueuedGenerator(fake, 1)

	_, err := generator.Generate(context.Background(), GenerateRequest{ImageIndex: 1})
	if !errors.Is(err, backendErr) {
		t.Fatalf("expected backend error, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = generator.Generate(ctx, GenerateRequest{ImageIndex: 2})
	if err != nil {
		t.Fatalf("expected second Generate call to enter after backend error, got %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 backend calls, got %d", fake.calls)
	}
}

func TestQueuedGeneratorDefaultsToTenForNonPositiveLimit(t *testing.T) {
	fake := newBlockingQueuedTestGenerator(11)
	generator := NewQueuedGenerator(fake, 0)

	var wg sync.WaitGroup
	for i := 1; i <= 11; i++ {
		index := i
		wg.Go(func() {
			_, _ = generator.Generate(context.Background(), GenerateRequest{ImageIndex: index})
		})
	}

	for range 10 {
		waitForQueuedTestEntry(t, fake.entered)
	}
	assertNoQueuedTestEntry(t, fake.entered)

	close(fake.release)
	waitForQueuedTestWorkers(t, &wg)
}

type blockingQueuedTestGenerator struct {
	entered chan GenerateRequest
	release chan struct{}
}

func newBlockingQueuedTestGenerator(enteredBuffer int) *blockingQueuedTestGenerator {
	return &blockingQueuedTestGenerator{
		entered: make(chan GenerateRequest, enteredBuffer),
		release: make(chan struct{}),
	}
}

func (g *blockingQueuedTestGenerator) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	select {
	case g.entered <- req:
	case <-ctx.Done():
		return GenerateResult{}, ctx.Err()
	}
	select {
	case <-g.release:
		return GenerateResult{Path: "ok.png"}, nil
	case <-ctx.Done():
		return GenerateResult{}, ctx.Err()
	}
}

type scriptedQueuedTestGenerator struct {
	calls  int
	errors []error
}

func (g *scriptedQueuedTestGenerator) Generate(context.Context, GenerateRequest) (GenerateResult, error) {
	g.calls++
	if len(g.errors) >= g.calls && g.errors[g.calls-1] != nil {
		return GenerateResult{}, g.errors[g.calls-1]
	}
	return GenerateResult{Path: "ok.png"}, nil
}

func waitForQueuedTestEntry(t *testing.T, entered <-chan GenerateRequest) GenerateRequest {
	t.Helper()
	select {
	case req := <-entered:
		return req
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for backend Generate call")
		return GenerateRequest{}
	}
}

func assertNoQueuedTestEntry(t *testing.T, entered <-chan GenerateRequest) {
	t.Helper()
	select {
	case req := <-entered:
		t.Fatalf("unexpected backend Generate call for image index %d", req.ImageIndex)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitForQueuedTestWorkers(t *testing.T, wg *sync.WaitGroup) {
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
