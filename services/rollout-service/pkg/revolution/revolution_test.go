/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/
package revolution

import (
	"context"
	"testing"

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
)

func TestSubscriber(t *testing.T) {
	type step struct {
	}
	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name:  "works",
			Steps: []step{},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			bc := service.New()
			errCh := make(chan error, 1)
			cs := New(Config{})
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				errCh <- cs.Subscribe(ctx, bc)
			}()
      for i, s := range tc.Steps {

      }
			cancel()
			err := <-errCh
			if err != nil {
				t.Errorf("expected no error but got %q", err)
			}
		})
	}
}
