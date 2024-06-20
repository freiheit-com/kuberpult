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

package time

import (
	"context"
	"time"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
)

func GetTimeNow(ctx context.Context) time.Time {
	t, ok := ctx.Value(ctxMarkerKey).(time.Time)
	if !ok {
		panic("no time in context")
	}
	return t
}

func WithTimeNow(ctx context.Context, t time.Time) context.Context {
	if _, ok := ctx.Value(ctxMarkerKey).(time.Time); ok {
		// already has time. used in testing
		return ctx
	}
	return context.WithValue(ctx, ctxMarkerKey, t)
}
