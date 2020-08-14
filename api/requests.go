package api

import (
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type ServiceDTO struct {
	Ports nat.PortSet
}

type InstanceDTO struct {
	Static          bool
	PortTranslation nat.PortMap `json:"port_translation"`
	Local           bool
}

type DiscoverDTO struct {
	MessageId          uuid.UUID
	Host               string
	HostAddr           string
	NeighborSent       string
	Services           map[string]*Service
	ServiceToInstances map[string][]string
	Instances          map[string]*Instance
	Hops               int
	MaxHops            int
}

type NeighborDTO struct {
	Addr string
}

type ToResolveDTO struct {
	Host string
	Port nat.Port
}
