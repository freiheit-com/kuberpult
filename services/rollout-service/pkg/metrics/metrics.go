package metrics

import (
	"context"
	"time"

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)


func Metrics(ctx context.Context, bc *service.Broadcast, meter metric.MeterProvider) error {
	st, ch, unsub := bc.Start()
	defer unsub()
	state := map[string]
	for _, ev := range st {
	    state[fmt.Sprintf("%s-%s",ev.Environment,ev.Application)] = struct{}
	}
	return nil
}
