package api

import (
	"github.com/docker/go-connections/nat"
)

type ServiceDTO struct {
	Dependencies []string
	Ports        nat.PortSet
}

type InstanceDTO struct {
	Static          bool
	PortTranslation nat.PortMap `json:"port_translation"`
}
