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

package service

import (
	"context"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"google.golang.org/grpc"
)

type BatchServiceWithDefaultTimeout struct {
	Inner          api.BatchServiceClient
	DefaultTimeout time.Duration
}

func (b *BatchServiceWithDefaultTimeout) ProcessBatch(ctx context.Context, req *api.BatchRequest, options ...grpc.CallOption) (*api.BatchResponse, error) {
	var cancel context.CancelFunc
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, b.DefaultTimeout)
		defer cancel()
	}
	return b.Inner.ProcessBatch(ctx, req, options...)
}
