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

// For testing purposes only

type ErrMatcher struct {
	Message string
}

func (e ErrMatcher) Error() string {
	return e.Message
}

func (e ErrMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}
