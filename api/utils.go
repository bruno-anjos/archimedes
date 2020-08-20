package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

func ResolveServiceInArchimedes(httpClient *http.Client, hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		log.Error("hostport: ", hostPort)
		panic(err)
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	toResolve := ToResolveDTO{
		Host: host,
		Port: nat.Port(port + "/tcp"),
	}
	archReq := http_utils.BuildRequest(http.MethodPost, DefaultHostPort, GetResolvePath(), toResolve)

	resolved := ResolvedDTO{}
	status, _ := http_utils.DoRequest(httpClient, archReq, &resolved)

	switch status {
	case http.StatusNotFound:
		log.Debugf("could not resolve %s", hostPort)
		return hostPort, nil
	case http.StatusOK:
	default:
		return "", errors.New(
			fmt.Sprintf("got status %d while resolving %s in archimedes", status, hostPort))
	}

	resolvedHostPort := resolved.Host + ":" + resolved.Port

	log.Debugf("resolved %s to %s", hostPort, resolvedHostPort)

	return resolvedHostPort, nil
}
