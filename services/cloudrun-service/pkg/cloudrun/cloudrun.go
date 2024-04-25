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

package cloudrun

import (
	"context"
	"fmt"

	"google.golang.org/api/run/v1"
)

const (
	serviceLocationLabel = "cloud.googleapis.com/location"
)

type serviceConfigError struct {
	name             string
	namespaceMissing bool
	locationMissing  bool
}

func (p serviceConfigError) Error() string {
	if p.namespaceMissing {
		return fmt.Sprintf("Namespace is missing for service: %s", p.name)
	}
	if p.locationMissing {
		return fmt.Sprintf("location is missing for service: %s", p.name)
	}
	return "Configuration error"
}

func Deploy(ctx context.Context, svc *run.Service) error {
	serviceName := svc.Metadata.Name
	runService, err := run.NewService(ctx)
	if err != nil {
		return err
	}
	parent, err := getParent(svc)
	if err != nil {
		return err
	}
	req := runService.Projects.Locations.Services.List(parent)
	resp, err := req.Do()
	if err != nil {
		return err
	}
	if isPresent(resp.Items, serviceName) {
		runService.Projects.Locations.Services.ReplaceService(serviceName, svc)
	} else {
		runService.Projects.Locations.Services.Create(parent, svc)
	}
	return nil
}

func isPresent(services []*run.Service, serviceName string) bool {
	for _, service := range services {
		if service.Metadata.Name == serviceName {
			return true
		}
	}
	return false
}

func getParent(svc *run.Service) (string, error) {
	namespace := svc.Metadata.Namespace
	if namespace == "" {
		return "", serviceConfigError{name: svc.Metadata.Name, namespaceMissing: true}
	}
	location, exists := svc.Metadata.Labels[serviceLocationLabel]
	if !exists {
		return "", serviceConfigError{name: svc.Metadata.Name, locationMissing: true}
	}
	return fmt.Sprintf("projects/%s/locations/%s", namespace, location), nil
}
