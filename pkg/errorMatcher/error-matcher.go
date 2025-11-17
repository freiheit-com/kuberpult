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

package errorMatcher

import "strings"

// For testing purposes only

// ErrMatcher matches with "==", so only the exact error counts as a match
type ErrMatcher struct {
	Message string
}

func (e ErrMatcher) Error() string {
	return e.Message
}

func (e ErrMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

// ContainsErrMatcher matches with strings.Contains
// ContainsErrMatcher is only matching if ALL strings match
type ContainsErrMatcher struct {
	Messages []string
}

func (e ContainsErrMatcher) Error() string {
	return "[]{" + strings.Join(e.Messages, " , ") + "}"
}

func (e ContainsErrMatcher) Is(err error) bool {
	for _, msg := range e.Messages {
		if !strings.Contains(err.Error(), msg) {
			return false
		}
	}
	return true
}
