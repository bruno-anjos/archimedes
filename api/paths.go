package api

import (
	"fmt"
	"strconv"
)

// Paths
const (
	// TODO make this unexported probably

	PrefixPath = "/archimedes"

	ServicesPath             = "/services"
	ServicePath              = "/services/%s"
	ServiceInstancePath      = "/services/%s/%s"
	InstancePath             = "/instances/%s"
	DiscoverPath             = "/discover"
	WhoAreYouPath            = "/who"
	TablePath                = "/table"
	ResolvePath              = "/resolve"
)

const (
	StatusQueryVar = "status"

	StatusOutOfService = "OUT_OF_SERVICE"
	StatusUp           = "UP"
)

const (
	ArchimedesServiceName = "archimedes"
	Port                  = 50000
)

var (
	DefaultHostPort = ArchimedesServiceName + ":" + strconv.Itoa(Port)
)

func GetServicesPath() string {
	return PrefixPath + ServicesPath
}

func GetDiscoverPath() string {
	return PrefixPath + DiscoverPath
}

func GetWhoAreYouPath() string {
	return PrefixPath + WhoAreYouPath
}

func GetServicePath(serviceId string) string {
	return PrefixPath + fmt.Sprintf(ServicePath, serviceId)
}

func GetInstancePath(instanceId string) string {
	return PrefixPath + fmt.Sprintf(InstancePath, instanceId)
}

func GetServiceInstancePath(serviceId, instanceId string) string {
	return PrefixPath + fmt.Sprintf(ServiceInstancePath, serviceId, instanceId)
}

func GetTablePath() string {
	return PrefixPath + TablePath
}

func GetResolvePath() string {
	return PrefixPath + ResolvePath
}
