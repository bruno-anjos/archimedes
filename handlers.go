package main

import (
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"sync"

	"github.com/bruno-anjos/archimedes/api"
	scheduler "github.com/bruno-anjos/scheduler/api"
	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	maxHops = 2
)

var (
	messagesReceived sync.Map
	servicesTable    *ServicesTable
	archimedesId     string
)

func init() {
	messagesReceived = sync.Map{}

	servicesTable = NewServicesTable()

	archimedesId = uuid.New().String()

	log.Infof("ARCHIMEDES ID: %s", archimedesId)
}

func discoverHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in discoverService handler")

	discoverMsg := api.DiscoverMsg{}
	err := json.NewDecoder(r.Body).Decode(&discoverMsg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}

	_, ok := messagesReceived.Load(discoverMsg.MessageId)
	if ok {
		log.Debugf("repeated message %s, ignoring...", discoverMsg.MessageId)
		return
	}

	log.Debugf("got discover message %+v", discoverMsg)

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		panic(err)
	}

	preprocessMessage(remoteAddr, &discoverMsg)

	servicesTable.UpdateTableWithDiscoverMessage(discoverMsg.NeighborSent, &discoverMsg)

	messagesReceived.Store(discoverMsg.MessageId, struct{}{})

	postprocessMessage(&discoverMsg)
	broadcastMsgWithHorizon(&discoverMsg, maxHops)
}

func registerServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerService handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	serviceDTO := api.ServiceDTO{}
	err := json.NewDecoder(r.Body).Decode(&serviceDTO)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	service := &api.Service{
		Id:    serviceId,
		Ports: serviceDTO.Ports,
	}

	_, ok := servicesTable.GetService(serviceId)
	if ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	newTableEntry := &api.ServicesTableEntryDTO{
		Host:         archimedesId,
		HostAddr:     api.DefaultHostPort,
		Service:      service,
		Instances:    map[string]*api.Instance{},
		NumberOfHops: 0,
		MaxHops:      0,
		Version:      0,
	}

	servicesTable.AddService(serviceId, newTableEntry)
	sendServicesTable()

	log.Debugf("added service %s", serviceId)
}

func deleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteService handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	_, ok := servicesTable.GetService(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	servicesTable.DeleteService(serviceId)

	log.Debugf("deleted service %s", serviceId)
}

func registerServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	_, ok := servicesTable.GetService(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	instanceDTO := scheduler.InstanceDTO{}
	err := json.NewDecoder(r.Body).Decode(&instanceDTO)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ok = servicesTable.ServiceHasInstance(serviceId, instanceId)
	if ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	var host string
	if instanceDTO.Local {
		host = instanceId
	} else {
		host, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			panic(err)
		}
	}

	instance := &api.Instance{
		Id:              instanceId,
		Ip:              host,
		ServiceId:       serviceId,
		PortTranslation: instanceDTO.PortTranslation,
		Initialized:     instanceDTO.Static,
		Static:          false,
		Local:           instanceDTO.Local,
	}

	servicesTable.AddInstance(serviceId, instanceId, instance)
	sendServicesTable()
	log.Debugf("added instance %s to service %s", instanceId, serviceId)
}

func deleteServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)
	_, ok := servicesTable.GetService(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)
	instance, ok := servicesTable.GetServiceInstance(serviceId, instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	servicesTable.DeleteInstance(instance.ServiceId, instanceId)

	log.Debugf("deleted instance %s from service %s", instanceId, serviceId)
}

func getAllServicesHandler(w http.ResponseWriter, _ *http.Request) {
	log.Debug("handling request in getAllServices handler")

	http_utils.SendJSONReplyOK(w, servicesTable.GetAllServices())
}

func getAllServiceInstancesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllServiceInstances handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	_, ok := servicesTable.GetService(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http_utils.SendJSONReplyOK(w, servicesTable.GetAllServiceInstances(serviceId))
}

func getServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	instance, ok := servicesTable.GetServiceInstance(serviceId, instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http_utils.SendJSONReplyOK(w, instance)
}

func getInstanceHandler(w http.ResponseWriter, r *http.Request) {
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	instance, ok := servicesTable.GetInstance(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http_utils.SendJSONReplyOK(w, instance)
}

func whoAreYouHandler(w http.ResponseWriter, _ *http.Request) {
	log.Debug("handling whoAreYou request")
	http_utils.SendJSONReplyOK(w, archimedesId)
}

func getServicesTableHandler(w http.ResponseWriter, _ *http.Request) {
	http_utils.SendJSONReplyOK(w, servicesTable.ToDiscoverMsg(archimedesId))
}

func resolveHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("handling resolve request")

	toResolve := api.ToResolveDTO{}
	err := json.NewDecoder(r.Body).Decode(&toResolve)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	service, sOk := servicesTable.GetService(toResolve.Host)
	if !sOk {
		instance, iOk := servicesTable.GetInstance(toResolve.Host)
		if !iOk {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resolved, ok := resolveInstance(toResolve.Port, instance)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		http_utils.SendJSONReplyOK(w, resolved)
		return
	}

	instances := servicesTable.GetAllServiceInstances(service.Id)

	if len(instances) == 0 {
		log.Debugf("no instances for service %s", service.Id)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var randInstance *api.Instance
	randNum := rand.Intn(len(instances))
	for _, instance := range instances {
		if randNum == 0 {
			randInstance = instance
		} else {
			randNum--
		}
	}

	resolved, ok := resolveInstance(toResolve.Port, randInstance)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Debugf("resolved %s:%s to %s:%s", toResolve.Host, toResolve.Port.Port(), resolved.Host, resolved.Port)

	http_utils.SendJSONReplyOK(w, api.ResolvedDTO{
		Host: resolved.Host,
		Port: resolved.Port,
	})
}

var (
	allowedStatuses = map[string]struct{}{api.StatusOutOfService: {}, api.StatusUp: {}}
)

func changeInstanceStateHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in changeInstanceState handler")

	vars := mux.Vars(r)
	status := vars[api.StatusQueryVar]

	log.Debugf("status query param: %s", status)

	_, ok := allowedStatuses[status]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}

	// TODO
	panic("implement me")
}

func preprocessMessage(remoteAddr string, discoverMsg *api.DiscoverMsg) {
	for _, entry := range discoverMsg.Entries {
		if entry.Host == discoverMsg.NeighborSent {
			entry.HostAddr = remoteAddr
			for _, instance := range entry.Instances {
				instance.Ip = remoteAddr
			}
		}
	}
}

func postprocessMessage(discoverMsg *api.DiscoverMsg) {
	var servicesToDelete []string

	for serviceId, entry := range discoverMsg.Entries {
		if entry.NumberOfHops > maxHops {
			servicesToDelete = append(servicesToDelete, serviceId)
		}
	}

	for _, serviceToDelete := range servicesToDelete {
		delete(discoverMsg.Entries, serviceToDelete)
	}
}

func sendServicesTable() {
	discoverMsg := servicesTable.ToDiscoverMsg(archimedesId)
	if discoverMsg == nil {
		return
	}

	broadcastMsgWithHorizon(discoverMsg, maxHops)
}

func resolveInstance(originalPort nat.Port, instance *api.Instance) (*api.ResolvedDTO, bool) {
	if instance.Local {
		return &api.ResolvedDTO{
			Host: instance.Id,
			Port: originalPort.Port(),
		}, true
	} else {
		portNatResolved, ok := instance.PortTranslation[originalPort]
		if !ok {
			return nil, false
		}

		return &api.ResolvedDTO{
			Host: instance.Ip,
			Port: portNatResolved[0].HostPort,
		}, true
	}
}

func broadcastMsgWithHorizon(discoverMsg *api.DiscoverMsg, hops int) {
	// TODO this simulates the lower level layer
	return
}
