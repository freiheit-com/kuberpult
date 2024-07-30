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
	errorTypeGitRepo RetryErrorKind = iota
	//errorTypeTransaction                = iota
)

// RetryError implies that something failed that can and should be retried
type RetryError struct {
	OriginalError error
	ErrorType     RetryErrorKind
}

func (e *RetryError) Error() string {
	if e == nil {
		return ""
	}
	var msg = "unknown"
	switch e.ErrorType {
	case errorTypeGitRepo:
		msg = "git"
		//case errorTypeTransaction:
		//	msg = "transaction"
	}
	return fmt.Sprintf("retry error for kind '%s': %v", msg, e.OriginalError)
}

func IsRetryError(e error) (bool, *RetryError) {
	var re *RetryError
	if errors.As(e, &re) {
		return true, re
	}
	return false, nil
}

//func (e *RetryError) IsTransaction() bool {
//	return e.ErrorType == errorTypeTransaction
//}

func (e *RetryError) IsGitRepo() bool {
	return e.ErrorType == errorTypeGitRepo
}

func RetryGitRepo(originalError error) *RetryError {
	return &RetryError{
		OriginalError: originalError,
		ErrorType:     errorTypeGitRepo,
	}
}

func UnwrapUntilRetryError(err error) (*RetryError, bool) {
	for {
		var applyErr *RetryError
		if errors.As(err, &applyErr) {
			return applyErr, true
		}
		err2 := errors.Unwrap(err)
		if err2 == nil {
			// cannot unwrap any further
			return nil, false
		}
	}
}
