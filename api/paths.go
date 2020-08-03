package api

import (
	"fmt"
	"strconv"

	utils "github.com/bruno-anjos/solution-utils"
)

// Paths
const (
	PrefixPath = "/archimedes"

	ServicesPath             = "/services"
	ServicePath              = "/services/%s"
	ServiceInstancePath      = "/services/%s/%s"
	ServiceInstanceAlivePath = "/services/%s/%s/alive"
	InstancePath             = "/instances/%s"
)

const (
	StatusQueryVar = "status"

	StatusOutOfService = "OUT_OF_SERVICE"
	StatusUp           = "UP"
)

const (
	Port = 50000
)

var (
	DefaultHostPort = utils.DefaultInterface + ":" + strconv.Itoa(Port)
)

func GetServicePath(serviceId string) string {
	return PrefixPath + fmt.Sprintf(ServicePath, serviceId)
}

func GetInstancePath(instanceId string) string {
	return PrefixPath + fmt.Sprintf(InstancePath, instanceId)
}

func GetServiceInstancePath(serviceId, instanceId string) string {
	return PrefixPath + fmt.Sprintf(ServiceInstancePath, serviceId, instanceId)
}

func GetServiceInstanceAlivePath(serviceId, instanceId string) string {
	return PrefixPath + fmt.Sprintf(ServiceInstanceAlivePath, serviceId, instanceId)
}
