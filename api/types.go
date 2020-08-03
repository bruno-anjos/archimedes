package api

import (
	"sync"

	"github.com/docker/go-connections/nat"
)

type (
	TypeInstancesMapKey   = string
	TypeInstancesMapValue = *Instance

	Service struct {
		Id           string
		Ports        nat.PortSet
		InstancesMap *sync.Map
	}
)

type Instance struct {
	Id              string
	Service         *Service
	Ip              string
	PortTranslation nat.PortMap
	Initialized     bool
}

type PairServiceIdStatus struct {
	ServiceId string
	IsUp      bool
	Mutex     *sync.Mutex
}
