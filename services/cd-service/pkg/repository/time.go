/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
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
