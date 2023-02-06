
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
