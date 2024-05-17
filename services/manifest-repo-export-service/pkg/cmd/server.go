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

package cmd

import (
	"context"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func RunServer() {
	err := logger.Wrap(context.Background(), func(ctx context.Context) error {
		logger.FromContext(ctx).Sugar().Warnf("hello world from the manifest-repo-export-service!")
		return nil
	})
	if err != nil {
		fmt.Printf("error in logger.wrap: %v %#v", err, err)
	}
}
