package api

import (
	"sync"

	"github.com/docker/go-connections/nat"
)

type (
	TypeInstancesMapKey   = string
	TypeInstancesMapValue = *Instance

	Service struct {
		Id      string
		Ports   nat.PortSet
	}
)

func (s *Service) ToTransfarable() *Service {
	return &Service{
		Id:      s.Id,
		Ports:   s.Ports,
	}
}

type Instance struct {
	Id              string
	ServiceId       string
	Ip              string
	PortTranslation nat.PortMap
	Initialized     bool
	Static          bool
	Local           bool
}

func (i *Instance) ToTransfarable() *Instance {
	return &Instance{
		Id:              i.Id,
		ServiceId:       i.ServiceId,
		Ip:              "",
		PortTranslation: i.PortTranslation,
		Initialized:     i.Initialized,
		Static:          i.Static,
		Local:           false,
	}
}

func FromTransfarableInstance(i *Instance, originIp string) *Instance {
	return &Instance{
		Id:              i.Id,
		ServiceId:       i.ServiceId,
		Ip:              originIp,
		PortTranslation: i.PortTranslation,
		Initialized:     i.Initialized,
		Static:          i.Static,
		Local:           false,
	}
}

type PairServiceIdStatus struct {
	ServiceId string
	IsUp      bool
	Mutex     *sync.Mutex
}
