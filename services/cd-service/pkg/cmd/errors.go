package cmd

import "fmt"

type releaseVersionsLimitError struct {
	limit uint
}

func (s releaseVersionsLimitError) Error() string {
	return fmt.Sprintf("releaseVersionsLimit: %d, must be between %d and %d", s.limit, minReleaseVersionsLimit, maxReleaseVersionsLimit)
}

type deploymentTypeConfigError struct {
	deploymentTypeInvalid bool
	cloudrunServerMissing bool
}

func (s deploymentTypeConfigError) Error() string {
	if s.deploymentTypeInvalid {
		return "invalid deploymentType. must either be k8s or cloudrun"
	}
	if s.cloudrunServerMissing {
		return "CloudRunServer cannot be empty"
	}
	return "invalid configuration"
}
