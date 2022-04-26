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
package testlogger

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func Wrap(ctx context.Context, inner func(ctx context.Context)) *observer.ObservedLogs {
	config, obs := observer.New(zap.DebugLevel)
	log := zap.New(config)
	defer func() {
		if err := log.Sync(); err != nil {
			panic(err)
		}
	}()
	inner(logger.WithLogger(ctx, log))
	return obs
}

func AssertEmpty(t *testing.T, logs *observer.ObservedLogs) {
	l := logs.All()
	if len(l) != 0 {
		t.Errorf("expected no logs, but got %d\nexample: \t%#v", len(l), l[0])
	}
}

type LogAssertion func(*testing.T, observer.LoggedEntry)

func AssertLogs(t *testing.T, logs *observer.ObservedLogs, tests ...LogAssertion) {
	l := logs.All()
	if len(l) > len(tests) {
		t.Errorf("expected %d logs, but got %d\nexample: \t%#v", len(tests), len(l), l[len(tests)])
	} else if len(l) < len(tests) {
		t.Errorf("expected %d logs, but got %d", len(tests), len(l))
	} else {
		for i, j := range l {
			tests[i](t, j)
		}
	}
}
