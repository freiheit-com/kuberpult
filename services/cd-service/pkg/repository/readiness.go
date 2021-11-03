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
	"sync"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
)

type Readiness struct {
	c     *sync.Cond
	ready bool
	err   error
}

func (r *Readiness) IsReady() (bool, error) {
	r.c.L.Lock()
	defer r.c.L.Unlock()
	return r.ready, r.err
}

func (r *Readiness) WaitReady() error {
	r.c.L.Lock()
	defer r.c.L.Unlock()
	for !r.ready {
		r.c.Wait()
	}
	return r.err
}

func (r *Readiness) setReady(ctx context.Context, fn func() error) {
	logger := logger.FromContext(ctx)
	err := fn()
	if err != nil {
		logger.Error("readiness.failed", zap.Error(err))
	} else {
		logger.Debug("readiness.ready")
	}
	r.c.L.Lock()
	defer r.c.L.Unlock()
	r.ready = true
	r.err = err
	r.c.Broadcast()
}

func newReadiness() *Readiness {
	return &Readiness{
		c: sync.NewCond(&sync.Mutex{}),
	}
}
