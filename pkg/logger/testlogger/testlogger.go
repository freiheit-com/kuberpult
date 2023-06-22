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

package testlogger

import (
	"context"
	"go.uber.org/zap/zapcore"
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

// AssertEmpty checks that there are no error logs. Lower level logs (warn, info, debug) are ignored.
func AssertEmpty(t *testing.T, logs *observer.ObservedLogs) {
	l := logs.All()
	errorLogs := make([]observer.LoggedEntry, 0)
	for i := range l {
		if l[i].Level >= zapcore.ErrorLevel {
			errorLogs = append(errorLogs, l[i])
		}
	}
	if len(errorLogs) != 0 {
		t.Errorf("expected no logs, but got %d\nexample: \t%#v", len(errorLogs), errorLogs[0])
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
