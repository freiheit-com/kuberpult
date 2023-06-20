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

package cmd

import (
	"context"
	"testing"
)

func TestService(t *testing.T) {
	tcs := []struct {
		Name          string
		ExpectedError string
	}{
		{
			Name:          "simple case",
			ExpectedError: "connecting to argocd : Argo CD server address unspecified",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			err := runServer(ctx, Config{})
			if err != nil {
				if err.Error() != tc.ExpectedError {
					t.Errorf("expected error %q but got %q", tc.ExpectedError, err)
				}
			} else if tc.ExpectedError != "" {
				t.Errorf("expected error %q but got <nil>", tc.ExpectedError)
			}
		})
	}
}
