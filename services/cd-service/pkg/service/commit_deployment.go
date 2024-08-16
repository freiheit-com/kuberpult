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

Copyright freiheit.com*/

package service

import (
	"context"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

type CommitDeploymentServer struct{}

func (_ *CommitDeploymentServer) GetCommitDeploymentInfo(ctx context.Context, in *api.GetCommitDeploymentInfoRequest) (*api.GetCommitDeploymentInfoResponse, error) {
	return &api.GetCommitDeploymentInfoResponse{}, nil
}
