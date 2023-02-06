
package repository

import (
	"context"
	"time"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
)

func getTimeNow(ctx context.Context) time.Time {
	t, ok := ctx.Value(ctxMarkerKey).(time.Time)
	if !ok {
		panic("no time in context")
	}
	return t
}

func withTimeNow(ctx context.Context, t time.Time) context.Context {
	if _, ok := ctx.Value(ctxMarkerKey).(time.Time); ok {
		// already has time. used in testing
		return ctx
	}
	return context.WithValue(ctx, ctxMarkerKey, t)
}
