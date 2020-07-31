package main

import (
	"sync"

	"github.com/bruno-anjos/archimedes/api"
)

type (
	typeInstancesMapKey   = string
	typeInstancesMapValue = *Instance

	Service struct {
		Id           string
		ServiceDTO   *api.ServiceDTO
		InstancesMap *sync.Map
	}
)

type Instance struct {
	Id          string
	Service     *Service
	Ip          string
	InstanceDTO *api.InstanceDTO
	Initialized bool
	InitChan    chan struct{}
}

type PairServiceIdStatus struct {
	ServiceId string
	IsUp      bool
	Mutex     *sync.Mutex
}
