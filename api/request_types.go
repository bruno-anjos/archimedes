package api

import (
	"github.com/docker/go-connections/nat"
)

type ServiceDTO struct {
	Dependencies []string
	Ports        nat.PortSet
}

type InstanceDTO struct {
	PortTranslation nat.PortMap
}
