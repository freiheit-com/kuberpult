
package repository

import "context"

type queue struct {
	elements chan element
}

type element struct {
	ctx          context.Context
	transformers []Transformer
	result       chan error
}

func (q *queue) add(ctx context.Context, transformers []Transformer) <-chan error {
	resultChannel := make(chan error, 1)
	e := element{
		ctx:          ctx,
		transformers: transformers,
		result:       resultChannel,
	}
	select {
	case q.elements <- e:
		return resultChannel
	case <-ctx.Done():
		resultChannel <- ctx.Err()
		return resultChannel
	}
}

func makeQueue() queue {
	return queue{
		elements: make(chan element, 5),
	}
}
