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
		Dependencies []string
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
	InitChan        chan struct{}
}

type PairServiceIdStatus struct {
	ServiceId string
	IsUp      bool
	Mutex     *sync.Mutex
}
