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

Copyright freiheit.com*/

package service

import (
	"context"
	"errors"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type BatchServiceWithDefaultTimeout struct {
	Inner          api.BatchServiceClient
	DefaultTimeout time.Duration
}

func (b *BatchServiceWithDefaultTimeout) ProcessBatch(ctx context.Context, req *api.BatchRequest, options ...grpc.CallOption) (*api.BatchResponse, error) {
	var cancel context.CancelFunc
	_, hasDeadline := ctx.Deadline()
	kuberpultTimeoutError := errors.New("kuberpult batch client timeout exceeded")
	if !hasDeadline {
		ctx, cancel = context.WithTimeoutCause(ctx, b.DefaultTimeout, kuberpultTimeoutError)
		defer cancel()
	}

	response, err := b.Inner.ProcessBatch(ctx, req, options...)

	if ctx.Err() != nil {
		if context.Cause(ctx) == kuberpultTimeoutError {
			logger.FromContext(ctx).Warn("Context cancelled due to kuberpult timeout")
		} else {
			logger.FromContext(ctx).Sugar().Warnf("Context cancelled due %v", zap.Error(context.Cause(ctx)))
		}
	}

	return response, err
}
