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

	archReq := http_utils.BuildRequest(http.MethodGet, DefaultHostPort, GetServicePath(host), nil)

	status, resp := http_utils.DoRequest(httpClient, archReq, nil)

	switch status {
	case http.StatusNotFound:
		log.Debugf("could not resolve service %s", hostPort)
		resolvedHostPort, err = resolveInstanceInArchimedes(httpClient, hostPort)
		if err != nil {
			return "", err
		}
		return resolvedHostPort, nil
	case http.StatusOK:
	default:
		return "", errors.New(
			fmt.Sprintf("got status %d while resolving %s in archimedes", resp.StatusCode, hostPort))
	}

	var instances map[string]*Instance
	err = json.NewDecoder(resp.Body).Decode(&instances)
	if err != nil {
		panic(err)
	}

	log.Debugf("got instances %+v", instances)

	portWithProto, err := nat.NewPort(genericutils.TCP, port)
	if err != nil {
		panic(err)
	}

	var randInstance *Instance
	randNum := rand.Intn(len(instances))

	for _, instance := range instances {
		if randNum == 0 {
			randInstance = instance
		} else {
			randNum--
		}
	}

	if randInstance == nil {
		log.Fatalf("could not find instance for service %s", host)
		return
	}

	var portResolved string
	if randInstance.Local {
		portResolved = portWithProto.Port()
	} else {
		portNATResolved, ok := randInstance.PortTranslation[portWithProto]
		if !ok {
			log.Fatalf("instance %s does not have mapping for port %d", host, portWithProto)
		}
		portResolved = portNATResolved[0].HostPort
	}

	resolvedHostPort = randInstance.Ip + ":" + portResolved

	log.Debugf("resolved %s to %s", hostPort, resolvedHostPort)

	return resolvedHostPort, nil
}

func resolveInstanceInArchimedes(httpClient *http.Client, hostPort string) (resolvedHostPort string, err error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		panic(err)
	}

	archReq := http_utils.BuildRequest(http.MethodGet, DefaultHostPort, GetInstancePath(host), nil)

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

	var instance Instance
	err = json.NewDecoder(resp.Body).Decode(&instance)
	if err != nil {
		panic(err)
	}

	log.Debugf("got instance %+v", instance)

	portWithProto, err := nat.NewPort(genericutils.TCP, port)
	if err != nil {
		panic(err)
	}

	var portResolved string
	if instance.Local {
		portResolved = portWithProto.Port()
	} else {
		portNATResolved, ok := instance.PortTranslation[portWithProto]
		if !ok {
			log.Fatalf("instance %s does not have mapping for port %d", host, portWithProto)
		}
		portResolved = portNATResolved[0].HostPort
	}

	resolvedHostPort = instance.Ip + ":" + portResolved

	log.Debugf("resolved %s to %s", hostPort, resolvedHostPort)

	return resolvedHostPort, nil
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
