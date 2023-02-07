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

package handler

import (
	"context"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func handleGRPCError(ctx context.Context, w http.ResponseWriter, err error) {
	s, _ := status.FromError(err)
	switch s.Code() {
	case codes.InvalidArgument:
		http.Error(w, s.Message(), http.StatusBadRequest)
	default:
		logger.FromContext(ctx).Error(s.Message())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
