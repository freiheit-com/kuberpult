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

Copyright 2023 freiheit.com*/

package testrepository

import (
	"context"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	git "github.com/libgit2/git2go/v34"
)

func Failing(err error) repository.Repository {
	//exhaustruct:ignore
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

func (fr *failingRepository) ApplyTransformersInternal(ctx context.Context, transformers ...repository.Transformer) ([]string, *repository.State, []*repository.TransformerResult, error) {
	return nil, nil, nil, fr.err
}

func (fr *failingRepository) State() *repository.State {
	//exhaustruct:ignore
	return &repository.State{}
}

func (fr *failingRepository) StateAt(oid *git.Oid) (*repository.State, error) {
	//exhaustruct:ignore
	return &repository.State{}, nil
}

func (fr *failingRepository) Notify() *notify.Notify {
	return &fr.notify
}
