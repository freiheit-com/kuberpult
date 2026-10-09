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

package testfs

import (
	"sync"

	"github.com/go-git/go-billy/v5"
)

type Operation int

type FileOperation struct {
	Operation Operation
	Filename  string
}

type value struct {
	errored bool
}

// UsageCollector tracks which file operations have been used in a test suite and reports which were not
// tested with an injected error.
type UsageCollector struct {
	mx    sync.Mutex
	usage map[FileOperation]value
}

type errorInjector struct {
	operation Operation
	filename  string
	err       error
	used      bool
	collector *UsageCollector
}

// A special filesystem that allows injecting errors at arbitrary operations.
type Filesystem struct {
	Inner         billy.Filesystem
	errorInjector errorInjector
}
