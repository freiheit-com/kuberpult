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

package cloudrun

import "fmt"

type ServiceReadyCondition struct {
	Name     string
	Revision int64
	Status   string
	Reason   string
	Message  string
}

func (c ServiceReadyCondition) String() string {
	return fmt.Sprintf("Service:%s, ObservedGeneration:%d, Ready:%s, Reason:%s, Message:%s", c.Name, c.Revision, c.Status, c.Reason, c.Message)
}

type serviceManifestError struct {
	metadataMissing bool
	nameEmpty       bool
}

func (p serviceManifestError) Error() string {
	if p.metadataMissing {
		return "No metadata found in service manifest"
	}
	if p.nameEmpty {
		return "Service name cannot be empty"
	}
	return "Configuration error"
}

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

type operationIdMissingError struct {
	name string
}

func (p operationIdMissingError) Error() string {
	return fmt.Sprintf("failed to get operation-id for service %s", p.name)
}

type serviceReadyConditionError struct {
	name string
}

func (p serviceReadyConditionError) Error() string {
	return fmt.Sprintf("failed to get Ready status for service %s", p.name)
}
