/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package repository

import "context"

type queue struct {
	elements chan element
}

type element struct {
	ctx          context.Context
	transformers []Transformer
	result       chan error
}

func (q *queue) add(ctx context.Context, transformers []Transformer) <-chan error {
	resultChannel := make(chan error, 1)
	e := element{
		ctx:          ctx,
		transformers: transformers,
		result:       resultChannel,
	}
	select {
	case q.elements <- e:
		return resultChannel
	case <-ctx.Done():
		resultChannel <- ctx.Err()
		return resultChannel
	}
}

func makeQueue() queue {
	return queue{
		elements: make(chan element, 5),
	}
}
