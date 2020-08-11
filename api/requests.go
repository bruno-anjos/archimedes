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
	MessageId     uuid.UUID
	Host          string
	ServicesIds   []string
	Services      []*ServiceDTO
	Instances     map[string][]string
	InstancesDTOs map[string]*InstanceDTO
	ServicesHash  []byte
	Hops          int
	MaxHops       int
	Timestamp     string
}

type NeighborDTO struct {
	ArchimedesId string
	Addr         string
}
