package testrepository

import (
	"context"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func Failing(err error) repository.Repository {
	return &failingRepository{err}
}

type failingRepository struct {
	err error
}

func (fr *failingRepository) Apply(ctx context.Context, transformers ...repository.Transformer) error {
	return fr.err
}

func (fr *failingRepository) State() *repository.State {
	return &repository.State{}
}

func (fr *failingRepository) SetCallback(_ func(*repository.State)) {
}

func (fr *failingRepository) WaitReady() error {
	return nil
}

func (fr *failingRepository) IsReady() (bool, error) {
	return true, nil
}
