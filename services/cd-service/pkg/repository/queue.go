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
The queue here is used because applying a change to git (pushing) takes some time.
Still, every request waits for the transformer AND push to finish (that's what the `result` channel is for in the "element struct" below).
This queue improves the throughput when there are many parallel requests, because the "push" operation is done only once for multiple requests (a request here is essentially the same as a transformer).
Many parallel requests can happen in a CI with many microservices that all call the "release" endpoint almost at the same time.
This queue does not improve the latency, because each request still waits for the push to finish.
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
