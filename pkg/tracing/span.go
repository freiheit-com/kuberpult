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

package tracing

import (
	"context"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type OnErrFunc = func(err error) error

// StartSpanFromContext is the same as tracer.StartSpanFromContext, but also returns an onError function that tags the span as error
// You should call the onErrorFunc when the span should be marked as failed.
func StartSpanFromContext(ctx context.Context, name string) (tracer.Span, context.Context, OnErrFunc) {
	mySpan, ctx := tracer.StartSpanFromContext(ctx, name)
	onErr := func(err error) error {
		if err == nil {
			return nil
		}
		mySpan.Finish(tracer.WithError(err))
		return err
	}
	return mySpan, ctx, onErr
}
