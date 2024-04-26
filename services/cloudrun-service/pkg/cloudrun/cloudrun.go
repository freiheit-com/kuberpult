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
	serviceLocationLabel       = "cloud.googleapis.com/location"
	operationIdAnnotation      = "run.googleapis.com/operation-id"
	serviceReady               = "Ready"
	serviceConfigurationsReady = "ConfigurationsReady"
	serviceRoutesReady         = "RoutesReady"
)

var (
	runService *run.APIService
)

// Possible values are True, False or Unknown
type ServiceConditions struct {
	Ready              string
	ConfigurationReady string
	RoutesReady        string
}

func (c ServiceConditions) String() string {
	return fmt.Sprintf("Ready:%s, ConfigurationReady:%s, RoutesReady:%s", c.Ready, c.ConfigurationReady, c.Ready)
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

func Init(ctx context.Context) error {
	var err error
	runService, err = run.NewService(ctx)
	return err
}

func Deploy(ctx context.Context, svc *run.Service) error {
	serviceName := svc.Metadata.Name
	// Get the full path of the project. Example: projects/<project-id>/locations/<region>
	parent, err := getParent(svc)
	if err != nil {
		return err
	}
	req := runService.Projects.Locations.Services.List(parent)
	resp, err := req.Do()
	if err != nil {
		return err
	}
	var serviceCallResp *run.Service
	// If the service is already deployed before, then we need to call ReplaceService. Otherwise, we call Create.
	if isPresent(resp.Items, serviceName) {
		servicePath := fmt.Sprintf("%s/services/%s", parent, serviceName)
		serviceCall := runService.Projects.Locations.Services.ReplaceService(servicePath, svc)
		serviceCallResp, err = serviceCall.Do()
		if err != nil {
			return err
		}
	} else {
		serviceCall := runService.Projects.Locations.Services.Create(parent, svc)
		serviceCallResp, err = serviceCall.Do()
		if err != nil {
			return err
		}
	}
	return waitForOperation(parent, serviceCallResp, 60)
}

func GetServiceConditions(serviceCallResponse *run.Service) (ServiceConditions, error) {
	serviceConditions := ServiceConditions{}
	conditions := serviceCallResponse.Status.Conditions
	for _, condition := range conditions {
		switch condition.Type {
		case serviceConfigurationsReady:
			serviceConditions.ConfigurationReady = condition.Status
		case serviceReady:
			serviceConditions.Ready = condition.Status
		case serviceRoutesReady:
			serviceConditions.RoutesReady = condition.Status
		default:
			return serviceConditions, fmt.Errorf("unknown service condition type %s", condition.Type)
		}
	}
	return serviceConditions, nil
}

func getOperationId(parent string, serviceCallResp *run.Service) (string, error) {
	operationId, exists := serviceCallResp.Metadata.Annotations[operationIdAnnotation]
	if !exists {
		return "", fmt.Errorf("failed to get operation-id for service %s", serviceCallResp.Metadata.Name)
	}
	return fmt.Sprintf("%s/operations/%s", parent, operationId), nil
}

func waitForOperation(parent string, serviceCallResp *run.Service, timeoutSeconds uint16) error {
	operationId, err := getOperationId(parent, serviceCallResp)
	if err != nil {
		return err
	}
	opService := run.NewProjectsLocationsOperationsService(runService)
	waitOperationRequest := &run.GoogleLongrunningWaitOperationRequest{
		Timeout: fmt.Sprintf("%ds", timeoutSeconds),
	}
	opServiceCall := opService.Wait(operationId, waitOperationRequest)
	operationResp, err := opServiceCall.Do()
	if err != nil {
		return fmt.Errorf("failed to wait for the service %s: %s", serviceCallResp.Metadata.Name, err)
	}
	if !operationResp.Done {
		return fmt.Errorf("service %s creation exceeded the timeout of %d seconds", serviceCallResp.Metadata.Name, timeoutSeconds)
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
