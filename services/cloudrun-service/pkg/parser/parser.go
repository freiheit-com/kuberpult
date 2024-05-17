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

package parser

import (
	"google.golang.org/api/run/v1"
	"sigs.k8s.io/yaml"
)

type InvalidServiceManifestError struct {
}

// ParseManifest umarshals a given YAML manifest and populates the provided service struct.
// Parameters:
//   - manifest: A byte slice containing the YAML data to be parsed.
//   - service: A pointer to the struct that will be populated with the data from the manifest.
func ParseManifest(manifest []byte, service *run.Service) error {
	err := yaml.Unmarshal(manifest, service)
	return err
}
