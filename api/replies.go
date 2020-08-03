package api

import (
	"github.com/docker/go-connections/nat"
)

type CompletedServiceDTO struct {
	Ports        nat.PortSet
	InstancesIds []TypeInstancesMapKey
	InstancesMap map[TypeInstancesMapKey]TypeInstancesMapValue
}

type CompletedInstanceDTO struct {
	Ports    nat.PortSet
	Instance *Instance
}
