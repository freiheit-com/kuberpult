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
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DeployServiceServer struct {
	Repository repository.Repository
}

func (d *DeployServiceServer) Deploy(
	ctx context.Context,
	in *api.DeployRequest,
) (*emptypb.Empty, error) {
	if err := ValidateDeployment(in.Environment, in.Application); err != nil {
		return nil, err
	}
	b := in.LockBehavior
	if in.IgnoreAllLocks {
		// the UI currently sets this to true,
		// in that case, we still want to ignore locks (for emergency deployments)
		b = api.LockBehavior_Ignore
	}
	err := d.Repository.Apply(ctx, &repository.DeployApplicationVersion{
		Environment:   in.Environment,
		Application:   in.Application,
		Version:       in.Version,
		LockBehaviour: b,
	})

	if err != nil {
		var lockedErr *repository.LockedError
		if errors.As(err, &lockedErr) {
			detail := &api.LockedError{
				EnvironmentLocks:            map[string]*api.Lock{},
				EnvironmentApplicationLocks: map[string]*api.Lock{},
			}
			for k, v := range lockedErr.EnvironmentLocks {
				detail.EnvironmentLocks[k] = &api.Lock{
					Message: v.Message,
				}
			}
			for k, v := range lockedErr.EnvironmentApplicationLocks {
				detail.EnvironmentApplicationLocks[k] = &api.Lock{
					Message: v.Message,
				}
			}
			stat, sErr := status.New(codes.FailedPrecondition, "locked").WithDetails(detail)
			if sErr != nil {
				return nil, internalError(ctx, sErr)
			}
			return nil, stat.Err()
		}
		return nil, internalError(ctx, err)
	}

	return &emptypb.Empty{}, nil
}

func (d *DeployServiceServer) ReleaseTrain(
	ctx context.Context,
	in *api.ReleaseTrainRequest,
) (*api.ReleaseTrainResponse, error) {
	if !valid.EnvironmentName(in.Environment) {
		return nil, status.Error(codes.InvalidArgument, "invalid environment")
	}
	if in.Team != "" && !valid.TeamName(in.Team) {
		return nil, status.Error(codes.InvalidArgument, "invalid Team name")
	}
	err := d.Repository.Apply(ctx, &repository.ReleaseTrain{
		Environment: in.Environment,
		Team:        in.Team,
	})
	if err != nil {
		return nil, internalError(ctx, err)
	}
	state := d.Repository.State()
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return &api.ReleaseTrainResponse{}, fmt.Errorf("could not get environment config with state '%v': %w", state, err)
	}
	targetEnvName := in.Environment
	envConfigs, ok := configs[targetEnvName]
	if !ok {
		return &api.ReleaseTrainResponse{}, fmt.Errorf("could not find environment config for '%v'", targetEnvName)
	}
	if envConfigs.Upstream == nil {
		return &api.ReleaseTrainResponse{}, nil
	}

	upstreamLatest := envConfigs.Upstream.Latest
	upstreamEnvName := envConfigs.Upstream.Environment

	if !upstreamLatest && upstreamEnvName == "" {
		return nil, fmt.Errorf("Environment %q does not have upstream.latest or upstream.environment configured - exiting.", targetEnvName)
	}
	if upstreamLatest && upstreamEnvName != "" {
		return nil, fmt.Errorf("Environment %q has both upstream.latest and upstream.environment configured - exiting.", targetEnvName)
	}

	upstream := upstreamEnvName
	if upstreamLatest {
		upstream = "latest"
	}

	return &api.ReleaseTrainResponse{Upstream: upstream, TargetEnv: targetEnvName}, nil
}

var _ api.DeployServiceServer = (*DeployServiceServer)(nil)
