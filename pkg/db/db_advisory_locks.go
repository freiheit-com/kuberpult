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

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type AdvisoryLockId int

type DBFunctionNoTx func(ctx context.Context) error

const (
	// LockIsolateTransformers is for transformer exclusivity:
	LockIsolateTransformers AdvisoryLockId = 666
)

func (h *DBHandler) WithAdvisoryLock(ctx context.Context, isShared bool, lockId AdvisoryLockId, f DBFunctionNoTx) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "WithAdvisoryLock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	err = h.DBAcquireAdvisoryLock(ctx, isShared, lockId)
	if err != nil {
		return err
	}
	err = f(ctx)
	if err != nil {
		unlockErr := h.DBReleaseAdvisoryLock(ctx, isShared, lockId)
		if unlockErr != nil {
			return fmt.Errorf("could not release advisory lock %v/%d and inner function failed: %w", isShared, lockId, errors.Join(err, unlockErr))
		} else {
			return fmt.Errorf("lock %v/%d was released, but the inner function failed with: %w", isShared, lockId, err)
		}
	} else {
		unlockErr := h.DBReleaseAdvisoryLock(ctx, isShared, lockId)
		if unlockErr != nil {
			return fmt.Errorf("could not release advisory lock %v/%d: %w", isShared, lockId, unlockErr)
		} else {
			return nil
		}
	}
}

// DBAcquireAdvisoryLock waits until the advisory lock is available
func (h *DBHandler) DBAcquireAdvisoryLock(ctx context.Context, isShared bool, lockID AdvisoryLockId) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBAcquireAdvisoryLock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("lockId", lockID)
	if h == nil {
		return fmt.Errorf("DBAcquireAdvisoryLock: no db handler provided")
	}

	err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		var selectQuery string
		if isShared {
			selectQuery = h.AdaptQuery("SELECT pg_advisory_xact_lock_shared(?)")
		} else {
			selectQuery = h.AdaptQuery("SELECT pg_advisory_xact_lock(?)")
		}
		span.SetTag("query", selectQuery)
		row, err := transaction.QueryContext(
			ctx,
			selectQuery,
			lockID,
		)
		if err != nil {
			return fmt.Errorf("could not query %s. Error: %w", selectQuery, err)
		}
		if !row.Next() {
			return fmt.Errorf("could not call Next on row: %w", err)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("DBAcquireAdvisoryLock: could not run transaction. Error: %w", err)
	}
	return nil
}

// DBReleaseAdvisoryLock releases the advisory lock immediately (not waiting for end of transaction)
func (h *DBHandler) DBReleaseAdvisoryLock(_ context.Context, _ bool, _ AdvisoryLockId) error {
	// with the transaction level locks, there is nothing to unlock.
	// closing the transaction will release all locks.
	return nil
}
