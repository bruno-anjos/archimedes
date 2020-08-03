package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	generic_utils "github.com/bruno-anjos/solution-utils"
	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

func ResolveServiceInArchimedes(hostPort, archimedesHostPort string) (resolvedHostPort string, err error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		panic(err)
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	archReq := http_utils.BuildRequest(http.MethodGet, archimedesHostPort, GetServicePath(host), nil)

	status, resp := http_utils.DoRequest(httpClient, archReq, nil)

	switch status {
	case http.StatusNotFound:
		log.Debugf("could not resolve service %s", hostPort)
		resolvedHostPort, err = resolveInstanceInArchimedes(httpClient, hostPort, archimedesHostPort)
		if err != nil {
			return "", err
		}
		return resolvedHostPort, nil
	case http.StatusOK:
	default:
		return "", errors.New(
			fmt.Sprintf("got status %d while resolving %s in archimedes", resp.StatusCode, hostPort))
	}

	var service CompletedServiceDTO
	err = json.NewDecoder(resp.Body).Decode(&service)
	if err != nil {
		panic(err)
	}

	portWithProto, err := nat.NewPort(generic_utils.TCP, port)
	if err != nil {
		panic(err)
	}

	_, ok := service.Ports[portWithProto]
	if !ok {
		return "", errors.New(fmt.Sprintf("port is not valid for service %s", host))
	}

	if len(service.InstancesIds) == 0 {
		panic(fmt.Sprintf("no instance for service %s", host))
	}

	randInstanceId := service.InstancesIds[rand.Intn(len(service.InstancesIds))]
	instance := service.InstancesMap[randInstanceId]
	portResolved := instance.PortTranslation[portWithProto][0]
	resolvedHostPort = instance.Ip + ":" + portResolved.HostPort

	log.Debugf("resolved %s to %s", hostPort, resolvedHostPort)

	return resolvedHostPort, nil
}

func resolveInstanceInArchimedes(httpClient *http.Client, hostPort,
	archimedesHostPort string) (resolvedHostPort string, err error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		panic(err)
	}

	archReq := http_utils.BuildRequest(http.MethodGet, archimedesHostPort, GetInstancePath(host), nil)

	status, resp := http_utils.DoRequest(httpClient, archReq, nil)
	switch status {
	case http.StatusNotFound:
		log.Debugf("could not resolve instance %s", hostPort)
		return hostPort, nil
	case http.StatusOK:
	default:
		return "", errors.New(
			fmt.Sprintf("got status %d while resolving %s in archimedes", status, hostPort))
	}

	var completedInstance CompletedInstanceDTO
	err = json.NewDecoder(resp.Body).Decode(&completedInstance)
	if err != nil {
		panic(err)
	}

	portWithProto, err := nat.NewPort(generic_utils.TCP, port)
	if err != nil {
		panic(err)
	}

	_, ok := completedInstance.Ports[portWithProto]
	if !ok {
		return "", errors.New(fmt.Sprintf("port is not valid for service %s", host))
	}

	portResolved := completedInstance.Instance.PortTranslation[portWithProto][0]
	resolvedHostPort = completedInstance.Instance.Ip + ":" + portResolved.HostPort

	log.Debugf("resolved %s to %s", hostPort, resolvedHostPort)

	return resolvedHostPort, nil
}

func SendHeartbeatInstanceToArchimedes(archimedesHostPort string) {
	serviceId := os.Getenv(generic_utils.ServiceEnvVarName)
	instanceId := os.Getenv(generic_utils.InstanceEnvVarName)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

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

	serviceInstancePath := GetServiceInstancePath(serviceId, instanceId)
	for {
		req = http_utils.BuildRequest(http.MethodPut, archimedesHostPort, serviceInstancePath, nil)
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
