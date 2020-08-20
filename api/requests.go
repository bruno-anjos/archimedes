package api

import (
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type ServiceDTO struct {
	Ports nat.PortSet
}

type ServicesTableEntryDTO struct {
	Host, HostAddr string
	Service        *Service
	Instances      map[string]*Instance
	NumberOfHops   int
	MaxHops        int
	Version        int
}
type DiscoverMsg struct {
	MessageId    uuid.UUID
	Origin       string
	NeighborSent string
	Entries      map[string]*ServicesTableEntryDTO
}

type NeighborDTO struct {
	Addr string
}

type ToResolveDTO struct {
	Host string
	Port nat.Port
}
