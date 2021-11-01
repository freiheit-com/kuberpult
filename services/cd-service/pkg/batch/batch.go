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
package batch

import (
	"context"
	"github.com/freiheit-com/fdc-continuous-delivery/pkg/api"
	"github.com/freiheit-com/fdc-continuous-delivery/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BatchServer struct {
	Repository *repository.Repository
}

func (d *BatchServer) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest,
) (*emptypb.Empty, error) {
	// your code goes here.

	return &emptypb.Empty{}, nil
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
