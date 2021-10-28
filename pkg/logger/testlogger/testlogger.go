package testlogger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func Start(ctx context.Context) (*observer.ObservedLogs, context.Context) {
	config, obs := observer.New(zap.DebugLevel)
	log := zap.New(config)
	return obs, logger.WithLogger(ctx, log)
}
