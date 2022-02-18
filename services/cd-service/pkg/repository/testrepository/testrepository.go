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
package testrepository

import (
	"context"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func Failing(err error) repository.Repository {
	return &failingRepository{err: err}
}

type failingRepository struct {
	err    error
	notify notify.Notify
}

func (fr *failingRepository) Apply(ctx context.Context, transformers ...repository.Transformer) error {
	return fr.err
}

func (fr *failingRepository) Push(ctx context.Context, pushAction func() error) error {
	return fr.err
}

func (fr *failingRepository) ApplyTransformersInternal(ctx context.Context, transformers ...repository.Transformer) ([]string, *repository.State, error) {
	return nil, nil, fr.err
}

func (fr *failingRepository) State() *repository.State {
	return &repository.State{}
}

func (fr *failingRepository) Notify() *notify.Notify {
	return &fr.notify
}
