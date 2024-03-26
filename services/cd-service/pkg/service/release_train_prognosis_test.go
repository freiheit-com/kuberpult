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

package service

import (
	"context"
	"errors"

	"testing"
	
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"google.golang.org/protobuf/proto"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestReleaseTrainPrognosis(t *testing.T) {
	type TestCase struct {
		Name             string
		Setup            []rp.Transformer
		Request          *api.ReleaseTrainRequest
		ExpectedResponse *api.GetReleaseTrainPrognosisResponse
		ExpectedError    error
	}

	tcs := []TestCase{
		{
			Name:             "First test",
			Setup:            []rp.Transformer{},
			Request:          &api.ReleaseTrainRequest{},
			ExpectedResponse: &api.GetReleaseTrainPrognosisResponse{},
			ExpectedError:    nil,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}

			err = repo.Apply(testutil.MakeTestContext(), tc.Setup...)
			if err != nil {
				t.Fatalf("error during setup, error: %v", err)
			}

			sv := &ReleaseTrainPrognosisServer{Repository: repo}
			resp, err := sv.GetReleaseTrainPrognosis(context.Background(), tc.Request)

			if !errors.Is(err, tc.ExpectedError) {
				t.Fatalf("expected error doesn't match actual error, expected %v, got %v", tc.ExpectedError, err)
			}
			if !proto.Equal(tc.ExpectedResponse, resp) {
				t.Fatalf("expected respones doesn't match actualy response, expected %v, got %v", tc.ExpectedResponse, resp)
			}
		})
	}
}
