package backend

import "context"

const defaultQueuedGeneratorConcurrency = 10

type queuedGenerator struct {
	generator Generator
	sem       chan struct{}
}

func NewQueuedGenerator(generator Generator, maxConcurrency int) Generator {
	if maxConcurrency <= 0 {
		maxConcurrency = defaultQueuedGeneratorConcurrency
	}
	return queuedGenerator{
		generator: generator,
		sem:       make(chan struct{}, maxConcurrency),
	}
}

func (g queuedGenerator) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return GenerateResult{}, ctx.Err()
	}
	defer func() { <-g.sem }()
	return g.generator.Generate(ctx, req)
}
