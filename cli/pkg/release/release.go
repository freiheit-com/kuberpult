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

package release

import (
	"fmt"
)

// a representation of the parameters of the /release endpoint
type ReleaseParameters struct {
	Application   string
	ManifestFiles map[string]string
}

// calls the Release endpoint with the specified parameters
// this function might be used in the future for programmatic interaction with Kuberpult, hence its separation
func Release(params *ReleaseParameters) error {
	if err := issueHttpRequest(params); err != nil {
		return fmt.Errorf("error while issuing HTTP request, error: %v", err)
	}
	return nil
}
