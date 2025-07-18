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

package backoff

import (
	"time"
)

type SimpleBackoff struct {
	initialDuration time.Duration
	nextDuration    time.Duration
	maxDuration     time.Duration
}

func MakeSimpleBackoff(initialDuration time.Duration, maxDuration time.Duration) SimpleBackoff {
	return SimpleBackoff{
		nextDuration:    initialDuration,
		initialDuration: initialDuration,
		maxDuration:     maxDuration,
	}
}

func (b *SimpleBackoff) NextBackOff() time.Duration {
	b.nextDuration = b.nextDuration * 2
	if b.nextDuration > b.maxDuration || b.nextDuration < 0 {
		b.nextDuration = b.maxDuration
	}
	return b.nextDuration
}

func (b *SimpleBackoff) Reset() {
	b.nextDuration = b.initialDuration
}

func (b *SimpleBackoff) IsAtMax() bool {
	return b.nextDuration == b.maxDuration
}
