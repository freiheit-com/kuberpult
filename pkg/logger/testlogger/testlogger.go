package testlogger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func Wrap(ctx context.Context, inner func(ctx context.Context) error) (*observer.ObservedLogs, error) {
	config, obs := observer.New(zap.DebugLevel)
	log := zap.New(config)
	defer func(){
		if err := log.Sync() ; err != nil {
			panic(err)
		}
	}()
	return obs, inner(logger.WithLogger(ctx, log))
}
