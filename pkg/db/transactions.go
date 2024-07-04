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
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DBFunction func(ctx context.Context, transaction *sql.Tx) error
type DBFunctionT[T any] func(ctx context.Context, transaction *sql.Tx) (*T, error)
type DBFunctionMultipleEntriesT[T any] func(ctx context.Context, transaction *sql.Tx) ([]T, error)

// WithTransaction opens a transaction, runs `f` and then calls either Commit or Rollback.
// Use this if the only thing to return from `f` is an error.
func (h *DBHandler) WithTransaction(ctx context.Context, readonly bool, f DBFunction) error {
	_, err := WithTransactionT(h, ctx, readonly, func(ctx context.Context, transaction *sql.Tx) (*interface{}, error) {
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
func WithTransactionT[T any](h *DBHandler, ctx context.Context, readonly bool, f DBFunctionT[T]) (*T, error) {
	res, err := WithTransactionMultipleEntriesT(h, ctx, readonly, func(ctx context.Context, transaction *sql.Tx) ([]T, error) {
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
	span, ctx := tracer.StartSpanFromContext(ctx, "DBTransaction")
	defer span.Finish()
	onError := func(e error) {
		span.Finish(tracer.WithError(e))
	}

	tx, err := h.DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelLinearizable,
		ReadOnly:  readonly,
	})
	if err != nil {
		onError(err)
		return nil, err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
		// we ignore the error returned from Rollback() here,
		// because it is always set when Commit() was successful
	}(tx)

	result, err := f(ctx, tx)
	if err != nil {
		onError(err)
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		onError(err)
		return nil, err
	}
	return result, nil
}
