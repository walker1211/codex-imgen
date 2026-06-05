package cli

import (
	"context"
	"sync"
	"time"

	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/result"
)

type LocalEngine struct {
	Generator   backend.Generator
	Prefix      string
	Prelude     string
	MaxAttempts int
	RetryDelays []time.Duration
}

func (e LocalEngine) RunSync(ctx context.Context, req SyncRequest) (result.Result, error) {
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 1
	}
	if req.Concurrency > req.Count {
		req.Concurrency = req.Count
	}

	prompt := codex.BuildPrompt(e.Prelude, e.Prefix, req.Prompt)

	images := make([]result.ImageResult, req.Count)
	sem := make(chan struct{}, req.Concurrency)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < req.Count; i++ {
		index := i
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			generated, err := e.generateWithRetry(ctx, prompt, req.Images)
			if err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
				return
			}
			images[index] = result.ImageResult{Index: index + 1, Status: "done", Path: generated.Path, URI: generated.URI}
		})
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return result.Result{}, err
	default:
	}
	return result.Result{OK: true, Prompt: req.Prompt, Status: "completed", Count: req.Count, Completed: len(images), Images: images}, nil
}

func (e LocalEngine) generateWithRetry(ctx context.Context, prompt string, images []string) (backend.GenerateResult, error) {
	attempts := e.maxAttempts()
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		generated, err := e.Generator.Generate(ctx, backend.GenerateRequest{Prompt: prompt, Images: images, Attempt: attempt})
		if err == nil {
			return generated, nil
		}
		lastErr = err
		if attempt >= attempts {
			break
		}
		delay := e.retryDelay(attempt)
		if delay <= 0 {
			continue
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return backend.GenerateResult{}, ctx.Err()
		}
	}
	return backend.GenerateResult{}, lastErr
}

func (e LocalEngine) maxAttempts() int {
	if e.MaxAttempts > 0 {
		return e.MaxAttempts
	}
	return 1
}

func (e LocalEngine) retryDelay(attempt int) time.Duration {
	delays := e.RetryDelays
	if len(delays) == 0 {
		delays = []time.Duration{5 * time.Second, 15 * time.Second}
	}
	if attempt <= 0 {
		return delays[0]
	}
	index := attempt - 1
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return delays[index]
}
