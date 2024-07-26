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
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/lib/pq"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"strings"
	"time"
)

const (
	DefaultNumRetries uint8 = 3 // number of retries, so number of total tries is always 1 more.
)

type DBFunction func(ctx context.Context, transaction *sql.Tx) error
type DBFunctionT[T any] func(ctx context.Context, transaction *sql.Tx) (*T, error)
type DBFunctionMultipleEntriesT[T any] func(ctx context.Context, transaction *sql.Tx) ([]T, error)

// WithTransaction opens a transaction, runs `f` and then calls either Commit or Rollback.
// Use this if the only thing to return from `f` is an error.
func (h *DBHandler) WithTransaction(ctx context.Context, readonly bool, f DBFunction) error {
	_, err := WithTransactionT(h, ctx, DefaultNumRetries, readonly, func(ctx context.Context, transaction *sql.Tx) (*interface{}, error) {
		err2 := f(ctx, transaction)
		if err2 != nil {
			return nil, err2
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// WithTransactionR works like WithTransaction but allows to specify the number of retries.
func (h *DBHandler) WithTransactionR(ctx context.Context, maxRetries uint8, readonly bool, f DBFunction) error {
	_, err := WithTransactionT(h, ctx, maxRetries, readonly, func(ctx context.Context, transaction *sql.Tx) (*interface{}, error) {
		err2 := f(ctx, transaction)
		if err2 != nil {
			return nil, err2
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// WithTransactionT is the same as WithTransaction, but you can also return data, not just the error.
func WithTransactionT[T any](h *DBHandler, ctx context.Context, maxRetries uint8, readonly bool, f DBFunctionT[T]) (*T, error) {
	res, err := WithTransactionMultipleEntriesRetryT(h, ctx, maxRetries, readonly, func(ctx context.Context, transaction *sql.Tx) ([]T, error) {
		fRes, err2 := f(ctx, transaction)
		if err2 != nil {
			return nil, err2
		}
		if fRes == nil {
			return make([]T, 0), nil
		}
		return []T{*fRes}, nil
	})
	if err != nil || len(res) == 0 {
		return nil, err
	}
	return &res[0], err
}

// WithTransactionMultipleEntriesT is the same as WithTransaction, but you can also return and array of data, not just the error.
func WithTransactionMultipleEntriesT[T any](h *DBHandler, ctx context.Context, readonly bool, f DBFunctionMultipleEntriesT[T]) ([]T, error) {
	return WithTransactionMultipleEntriesRetryT(h, ctx, DefaultNumRetries, readonly, f)
}

// WithTransactionMultipleEntriesRetryT is the same as WithTransaction, but you can also return and array of data and retries.
func WithTransactionMultipleEntriesRetryT[T any](h *DBHandler, ctx context.Context, maxRetries uint8, readonly bool, f DBFunctionMultipleEntriesT[T]) ([]T, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBTransaction")
	defer span.Finish()
	span.SetTag("readonly", readonly)
	span.SetTag("maxRetries", maxRetries)

	onError := func(e error) ([]T, error) {
		span.Finish(tracer.WithError(e))
		return nil, e
	}

	tx, err := h.BeginTransaction(ctx, readonly)

	retryMaybe := func(msg string, e error) ([]T, error) {
		if maxRetries == 0 {
			return onError(fmt.Errorf("error %s transaction: %w", msg, e))
		}
		if IsRetryableError(e) {
			duration := 250 * time.Millisecond
			logger.FromContext(ctx).Sugar().Warnf("%s transaction failed, will retry in %v: %v", msg, duration, e)
			_ = tx.Rollback()
			span.Finish()
			time.Sleep(duration)
			return WithTransactionMultipleEntriesRetryT(h, ctx, maxRetries-1, readonly, f)
		}
		return nil, e
	}

	if err != nil {
		return retryMaybe("beginning", err)
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
		// we ignore the error returned from Rollback() here,
		// because it is always set when Commit() was successful
	}(tx)

	result, err := f(ctx, tx)
	if err != nil {
		return retryMaybe("within", err)
	}
	err = tx.Commit()
	if err != nil {
		return retryMaybe("committing", err)
	}
	return result, nil
}

func (h *DBHandler) BeginTransaction(ctx context.Context, readonly bool) (*sql.Tx, error) {
	return h.DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
		ReadOnly:  readonly,
	})
}

func IsRetryableError(err error) bool {
	var pgErr = UnwrapUntilPostgresError(err)
	if pgErr == nil {
		// it's not even a postgres error, so we can't check if it's retryable
		return false
	}
	codeStr := string(pgErr.Code)
	// for a list of all postgres error codes, see https://www.postgresql.org/docs/9.3/errcodes-appendix.html
	if strings.HasPrefix(codeStr, "40") {
		return true
	}
	return false
}

func UnwrapUntilPostgresError(err error) *pq.Error {
	for {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			return pqErr
		}
		err2 := errors.Unwrap(err)
		if err2 == nil {
			// cannot unwrap any further
			return nil
		}
	}
}
