/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/

package repository

/**
This queue contains transformers. Do not confuse with the "queuedVersion" field in protobuf (api.proto).
The queue here is used because applying a change to git (pushing) takes some time,
but this violates general rest patterns (every endpoint should return quickly).
So we queue transformers, and don't wait for them to finish.
*/

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
