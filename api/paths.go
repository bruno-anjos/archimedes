package api

import (
	"fmt"
)

// Paths
const (
	PrefixPath = "/archimedes"

	ServicesPath        = "/services"
	ServicePath         = "/services/%s"
	ServiceInstancePath = "/services/%s/%s"
)

const (
	StatusQueryVar = "status"

	StatusOutOfService = "OUT_OF_SERVICE"
	StatusUp           = "UP"
)

func GetServicePath(serviceId string) string {
	return PrefixPath + fmt.Sprintf(ServicePath, serviceId)
}

func GetServiceInstancePath(serviceId, instanceId string) string {
	return PrefixPath + fmt.Sprintf(ServiceInstancePath, serviceId, instanceId)
}
