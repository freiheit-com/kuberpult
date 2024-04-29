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
