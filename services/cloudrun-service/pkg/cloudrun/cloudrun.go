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
	"log"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
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
		servicePath := fmt.Sprintf("%s/services/%s", parent, serviceName)
		serviceCall := runService.Projects.Locations.Services.ReplaceService(servicePath, svc)
		resp, err := serviceCall.Do()
		if err != nil {
			logger.FromContext(ctx).Error("Service replace failed", zap.String("Error", err.Error()))
			return err
		}
		operationId, err := getOperationId(parent, resp)
		if err != nil {
			logger.FromContext(ctx).Error("Failed to get operation id", zap.String("Error", err.Error()))
			return err
		}
		operation := runService.Projects.Locations.Operations.Wait(operationId, &run.GoogleLongrunningWaitOperationRequest{})
		opResp, err := operation.Do()
		if err != nil {
			log.Fatalf("Failed to get operation status: %v", err)
		}
		if !opResp.Done {
			logger.FromContext(ctx).Error("Service creation timed out")
		}
	} else {
		serviceCall := runService.Projects.Locations.Services.Create(parent, svc)
		_, err := serviceCall.Do()
		if err != nil {
			logger.FromContext(ctx).Error("Service create failed:", zap.String("Error", err.Error()))
			return err
		}

	}
	logger.FromContext(ctx).Info("Service deployed successfully")
	return nil
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
