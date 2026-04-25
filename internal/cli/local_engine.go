package cli

import (
	"context"
	"sync"

	"github.com/walker1211/codex-imgen/internal/backend"
	"github.com/walker1211/codex-imgen/internal/codex"
	"github.com/walker1211/codex-imgen/internal/result"
)

type LocalEngine struct {
	Generator backend.Generator
	Prefix    string
	Prelude   string
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

	prompts := make([]string, req.Count)
	for i := 0; i < req.Count; i++ {
		prompts[i] = codex.BuildPrompt(e.Prelude, e.Prefix, req.Prompt)
	}

	images := make([]result.ImageResult, req.Count)
	sem := make(chan struct{}, req.Concurrency)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < req.Count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			generated, err := e.Generator.Generate(ctx, backend.GenerateRequest{Prompt: prompts[index], Images: req.Images})
			if err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
				return
			}
			images[index] = result.ImageResult{Index: index + 1, Status: "done", Path: generated.Path, URI: generated.URI}
		}(i)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return result.Result{}, err
	default:
	}
	return result.Result{OK: true, Prompt: req.Prompt, Status: "completed", Count: req.Count, Completed: len(images), Images: images}, nil
}
