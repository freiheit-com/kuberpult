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
Still, every request waits for the transformer AND push to finish (that's what the `result` channel is for in the "transformerBatch struct" below).
This queue improves the throughput when there are many parallel requests, because the "push" operation is done only once for multiple requests (a request here is essentially the same as a transformer).
Many parallel requests can happen in a CI with many microservices that all call the "release" endpoint almost at the same time.
This queue does not improve the latency, because each request still waits for the push to finish.
*/

import (
	"context"
	"fmt"
)

type queue struct {
	transformerBatches chan transformerBatch
}

type transformerBatch struct {
	ctx          context.Context
	transformers []Transformer
	result       chan error
}

func (t *transformerBatch) finish(err error) {
	select {
	case t.result <- err:
		close(t.result)
	default:
	}
}

func (q *queue) add(ctx context.Context, transformers []Transformer) <-chan error {
	resultChannel := make(chan error, 1)
	e := transformerBatch{
		ctx:          ctx,
		transformers: transformers,
		result:       resultChannel,
	}

	select {
	case q.transformerBatches <- e:
		fmt.Printf("Length: %d", len(q.transformerBatches))
		return resultChannel
	case <-ctx.Done():
		e.finish(ctx.Err())
		return resultChannel
	default:
		//Channel is full, we don't want to put anything else there.
		e.finish(&TransformerBatchApplyError{
			Index:            -1,
			TransformerError: fmt.Errorf("Queue is full"),
		})
		return resultChannel
	}
}

func makeQueue() queue {
	return queue{
		transformerBatches: make(chan transformerBatch, 5),
	}
}

func makeQueueN(size int) queue {
	return queue{
		transformerBatches: make(chan transformerBatch, size),
	}
}
