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

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type EnvironmentServiceServer struct {
	Repository repository.Repository
	HealthCheckResult HealthCheckResultPtr
}

func (e *EnvironmentServiceServer) CreateEnvironment(
	ctx context.Context,
	in *api.CreateEnvironmentRequest) (*emptypb.Empty, error) {

	err := e.Repository.Apply(ctx, &repository.CreateEnvironment{
		Environment: in.Environment,
	})
	if err != nil {
		return nil, internalError(ctx, err, e.HealthCheckResult)
	}
	return &emptypb.Empty{}, nil
}
