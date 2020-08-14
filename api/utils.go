package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	genericutils "github.com/bruno-anjos/solution-utils"
	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

const (
	HeartbeatCheckerTimeout = 60
)

func ResolveServiceInArchimedes(httpClient *http.Client, hostPort string) (resolvedHostPort string, err error) {
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
	archReq := http_utils.BuildRequest(http.MethodGet, DefaultHostPort, GetResolvePath(), toResolve)

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

	return resolved.Host + ":" + resolved.Port, nil
}

func SendHeartbeatInstanceToArchimedes(archimedesHostPort string) {
	serviceId := os.Getenv(genericutils.ServiceEnvVarName)
	instanceId := os.Getenv(genericutils.InstanceEnvVarName)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	log.Infof("will start sending heartbeats to %s as %s from %s", archimedesHostPort, instanceId, serviceId)

	serviceInstanceAlivePath := GetServiceInstanceAlivePath(serviceId, instanceId)
	req := http_utils.BuildRequest(http.MethodPost, archimedesHostPort, serviceInstanceAlivePath, nil)
	status, _ := http_utils.DoRequest(httpClient, req, nil)

	switch status {
	case http.StatusConflict:
		log.Debugf("service %s instance %s already has a heartbeat sender", serviceId, instanceId)
		return
	case http.StatusOK:
	default:
		panic(errors.New(fmt.Sprintf("received unexpected status %d", status)))
	}

	ticker := time.NewTicker((HeartbeatCheckerTimeout / 3) * time.Second)
	serviceInstancePath := GetServiceInstancePath(serviceId, instanceId)
	req = http_utils.BuildRequest(http.MethodPut, archimedesHostPort, serviceInstancePath, nil)
	for {
		<-ticker.C
		log.Info("sending heartbeat to archimedes")
		status, _ = http_utils.DoRequest(httpClient, req, nil)

		switch status {
		case http.StatusNotFound:
			log.Warnf("heartbeat to archimedes retrieved not found")
		case http.StatusOK:
		default:
			panic(errors.New(fmt.Sprintf("received unexpected status %d", status)))
		}
	}
}
