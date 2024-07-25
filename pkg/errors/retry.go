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

package errors

import (
	"errors"
	"fmt"
)

type RetryErrorKind int

const (
	errorTypeGitRepo     RetryErrorKind = iota
	errorTypeTransaction                = iota
)

// RetryError implies that something failed that can and should be retried
type RetryError struct {
	originalError error
	errorType     RetryErrorKind
}

func (e *RetryError) Error() string {
	var msg = "unknown"
	switch e.errorType {
	case errorTypeGitRepo:
		msg = "git"
	case errorTypeTransaction:
		msg = "transaction"
	}
	return fmt.Sprintf("retry error for kind '%s': %v", msg, e.originalError)
}

func IsRetryError(e error) (bool, *RetryError) {
	var re *RetryError
	if errors.As(e, &re) {
		return true, re
	}
	return false, nil
}

func (e *RetryError) IsTransaction() bool {
	return e.errorType == errorTypeTransaction
}

func (e *RetryError) IsGitRepo() bool {
	return e.errorType == errorTypeGitRepo
}

func RetryGitRepo(originalError error) *RetryError {
	return &RetryError{
		originalError: originalError,
		errorType:     errorTypeGitRepo,
	}
}

func RetryTransaction(originalError error) *RetryError {
	return &RetryError{
		originalError: originalError,
		errorType:     errorTypeTransaction,
	}
}

func UnwrapUntilRetryError(err error) *RetryError {
	for {
		var applyErr *RetryError
		if errors.As(err, &applyErr) {
			return applyErr
		}
		err2 := errors.Unwrap(err)
		if err2 == nil {
			// cannot unwrap any further
			return nil
		}
	}
}
