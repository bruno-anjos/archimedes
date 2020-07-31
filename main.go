package main

import (
	"github.com/bruno-anjos/archimedes/api"
	utils "github.com/bruno-anjos/solution-utils"
)

const (
	serviceName = "ARCHIMEDES"
)

func main() {
	utils.StartServer(serviceName, api.DefaultHostPort, api.Port, api.PrefixPath, routes)
}
