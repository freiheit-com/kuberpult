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

func (q *queue) addElement(ctx context.Context, e element) <-chan error {
	ch := e.result
	select {
	case q.elements <- e:
		return ch
	case <-ctx.Done():
		ch <- ctx.Err()
		return ch
	}
}

func (q *queue) add(ctx context.Context, transformers []Transformer) <-chan error {
	ch := make(chan error, 1)
	e := element{
		ctx:          ctx,
		transformers: transformers,
		result:       ch,
	}
	return q.addElement(ctx, e)
}

func makeQueue() queue {
	return queue{
		elements: make(chan element, 5),
	}
}
