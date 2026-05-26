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

package kindbracketstest

import (
	"testing"
	"time"
)

func tLog(t *testing.T, args ...any) {
	t.Helper()
	t.Log(append([]any{"[" + time.Now().Format("15:04:05") + "]"}, args...)...)
}

func tLogf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("[%s] "+format, append([]any{time.Now().Format("15:04:05")}, args...)...)
}
