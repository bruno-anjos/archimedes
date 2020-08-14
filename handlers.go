package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bruno-anjos/archimedes/api"
	scheduler "github.com/bruno-anjos/scheduler/api"
	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type (
	typeHeartbeatsMapKey   = string
	typeHeartbeatsMapValue = *api.PairServiceIdStatus

	typeInitChansMapValue = chan struct{}

	typeNeighborsMapKey   = string
	typeNeighborsMapValue = *Neighbor

	Neighbor struct {
		ArchimedesId string
		Addr         string
	}
)

const (
	// in seconds
	httpClientTimeout = 20
	initTimeout       = 60
	maxHops           = 2
)

var (
	httpClient *http.Client

	heartbeatsMap sync.Map
	initChansMap  sync.Map

	neighbors sync.Map

	messagesReceived sync.Map

	servicesTable *ServicesTable

	archimedesId string
)

func init() {
	httpClient = &http.Client{
		Timeout: httpClientTimeout * time.Second,
	}

	heartbeatsMap = sync.Map{}
	initChansMap = sync.Map{}

	neighbors = sync.Map{}

	messagesReceived = sync.Map{}

	servicesTable = NewServicesTable()

	archimedesId = uuid.New().String()

	log.Infof("ARCHIMEDES ID: %s", archimedesId)

	go instanceHeartbeatChecker()
}

func addNeighborHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling add neighbor request")
	neighborDTO := &api.NeighborDTO{}
	err := json.NewDecoder(r.Body).Decode(neighborDTO)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		panic(err)
	}

	if neighborDTO.Addr == "" {
		panic("empty addr")
	}

	var nodeArchimedesId string
	req := http_utils.BuildRequest(http.MethodGet, neighborDTO.Addr, api.GetWhoAreYouPath(),
		nil)
	status, _ := http_utils.DoRequest(httpClient, req, &nodeArchimedesId)

	if status != http.StatusOK {
		log.Fatalf("got status code %d while asking archimedes node its id", status)
	}

	neighbor := &Neighbor{
		ArchimedesId: nodeArchimedesId,
		Addr:         neighborDTO.Addr,
	}

	log.Debugf("added neighbor %s in %s", neighbor.ArchimedesId, neighbor.Addr)

	neighbors.Store(neighbor.ArchimedesId, neighbor)
	sendServicesTableToNeighbor(neighbor)
}

func discoverHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in discoverService handler")

	discoverDTO := api.DiscoverDTO{}
	err := json.NewDecoder(r.Body).Decode(&discoverDTO)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		return
	}

	_, ok := messagesReceived.Load(discoverDTO.MessageId)
	if ok {
		log.Debugf("repeated message %s, ignoring...", discoverDTO.MessageId)
		return
	}

	log.Debugf("got discover message %+v", discoverDTO)

	remoteAddr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		panic(err)
	}

	preprocessMessage(remoteAddr, &discoverDTO)

	servicesTable.UpdateTableWithDiscoverMessage(discoverDTO.NeighborSent, &discoverDTO)

	messagesReceived.Store(discoverDTO.MessageId, struct{}{})

	postprocessMessage(&discoverDTO)
	go propagateMessageAsync(&discoverDTO)
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
		Id:      serviceId,
		Ports:   serviceDTO.Ports,
		Version: 0,
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

	instanceDTO := api.InstanceDTO{}
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

	initChan := make(chan struct{})
	instance := &api.Instance{
		Id:              instanceId,
		Ip:              host,
		ServiceId:       serviceId,
		PortTranslation: instanceDTO.PortTranslation,
		Initialized:     instanceDTO.Static,
		Static:          false,
		Local:           instanceDTO.Local,
	}

	initChansMap.Store(instanceId, initChan)
	servicesTable.AddInstance(serviceId, instanceId, instance)

	if !instanceDTO.Static {
		go cleanUnresponsiveInstance(serviceId, instanceId, initChan)
		log.Debugf("added INTERACTIVE instance %s to instances waiting for heartbeat", instanceId)
	} else {
		log.Debugf("added STATIC instance %s", instanceId)
	}

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

	removeInstance(instance.Id, instanceId)

	log.Debugf("deleted instance %s from service %s", instanceId, serviceId)
}

func heartbeatServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in heartbeatService handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	_, ok := servicesTable.GetService(serviceId)
	if !ok {
		log.Warnf("ignoring heartbeat since service %s wasn't registered", serviceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	instance, ok := servicesTable.GetServiceInstance(serviceId, instanceId)
	if !ok {
		log.Warnf("ignoring heartbeat from instance %s since it wasn't registered", instanceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !instance.Initialized {
		value, ok := initChansMap.Load(instance.Id)
		if !ok {
			log.Warnf("ignoring heartbeat from instance %s since it didnt have an init channel", instanceId)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		instance.Initialized = true
		initChan := value.(typeInitChansMapValue)
		close(initChan)
	}

	value, ok := heartbeatsMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pairServiceStatus := value.(typeHeartbeatsMapValue)
	pairServiceStatus.Mutex.Lock()
	pairServiceStatus.IsUp = true
	pairServiceStatus.Mutex.Unlock()

	log.Debugf("got heartbeat from instance %s", instanceId)
}

func registerHeartbeatServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	pairServiceStatus := &api.PairServiceIdStatus{
		ServiceId: serviceId,
		IsUp:      true,
		Mutex:     &sync.Mutex{},
	}

	_, loaded := heartbeatsMap.LoadOrStore(instanceId, pairServiceStatus)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
	}

	log.Debugf("registered service %s instance %s first heartbeat", serviceId, instanceId)
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

		resolved, ok := resolveInstance(toResolve.Host, toResolve.Port, instance)
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

	resolved, ok := resolveInstance(toResolve.Host, toResolve.Port, randInstance)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

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

func preprocessMessage(remoteAddr string, discoverMsg *api.DiscoverDTO) {
	for _, entry := range discoverMsg.Entries {
		if entry.Host == discoverMsg.NeighborSent {
			entry.HostAddr = remoteAddr
			for _, instance := range entry.Instances {
				instance.Ip = remoteAddr
			}
		}
	}
}

func postprocessMessage(discoverMsg *api.DiscoverDTO) {
	servicesToDelete := []string{}

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

	neighbors.Range(func(key, value interface{}) bool {
		neighbor := value.(typeNeighborsMapValue)

		req := http_utils.BuildRequest(http.MethodPost, neighbor.Addr, api.GetDiscoverPath(), discoverMsg)
		status, _ := http_utils.DoRequest(httpClient, req, nil)

		if status != http.StatusOK {
			log.Fatalf("got status %d", status)
		}

		return true
	})
}

func sendServicesTableToNeighbor(neighbor *Neighbor) {
	discoverMsg := servicesTable.ToDiscoverMsg(archimedesId)

	if discoverMsg == nil {
		return
	}

	req := http_utils.BuildRequest(http.MethodPost, neighbor.Addr, api.GetDiscoverPath(), discoverMsg)
	status, _ := http_utils.DoRequest(httpClient, req, nil)

	if status != http.StatusOK {
		log.Fatalf("got status %d", status)
	}
}

func removeInstance(serviceId, instanceId string) {
	servicesTable.DeleteInstance(serviceId, instanceId)
	heartbeatsMap.Delete(instanceId)
}

func propagateMessageAsync(discover *api.DiscoverDTO) {
	randInt := rand.Intn(500)
	time.Sleep(time.Duration(randInt) * time.Millisecond)

	originalNeighborSent := discover.NeighborSent
	discover.NeighborSent = archimedesId

	neighbors.Range(func(key, value interface{}) bool {
		neighborId := key.(typeNeighborsMapKey)

		if neighborId == discover.Origin {
			log.Debugf("not propagating message %s due to %s being the host", discover.MessageId, neighborId)
			return true
		}

		if neighborId == originalNeighborSent {
			log.Debugf("not propagating message %s due to %s being the neighbor that i received from",
				discover.MessageId, neighborId)
			return true
		}

		neighbor := value.(typeNeighborsMapValue)

		log.Debugf("propagating msg from %s to %s", discover.NeighborSent, neighborId)

		req := http_utils.BuildRequest(http.MethodPost, neighbor.Addr, api.GetDiscoverPath(), discover)
		status, _ := http_utils.DoRequest(httpClient, req, nil)

		if status != http.StatusOK {
			panic(fmt.Sprintf("got status %d while attempting to propagate", status))
		}

		return true
	})
}

func instanceHeartbeatChecker() {
	timer := time.NewTimer(api.HeartbeatCheckerTimeout * time.Second)

	var toDelete []string
	for {
		toDelete = []string{}
		<-timer.C
		log.Debug("checking heartbeats")
		heartbeatsMap.Range(func(key, value interface{}) bool {
			instanceId := key.(typeHeartbeatsMapKey)
			pairServiceStatus := value.(typeHeartbeatsMapValue)
			pairServiceStatus.Mutex.Lock()

			// case where instance didnt set online status since last status reset, so it has to be removed
			if !pairServiceStatus.IsUp {
				pairServiceStatus.Mutex.Unlock()
				_, ok := servicesTable.GetServiceInstance(pairServiceStatus.ServiceId, instanceId)
				// case where the instance will be removed and there is a service for that instance
				if ok {
					servicesTable.DeleteInstance(pairServiceStatus.ServiceId, instanceId)
				} else {
					log.Debugf("did not find instance %s in service %s, assuming it was already removed",
						instanceId, pairServiceStatus.ServiceId)
				}

				toDelete = append(toDelete, instanceId)
				log.Debugf("removing instance %s", instanceId)
			} else {
				pairServiceStatus.IsUp = false
				pairServiceStatus.Mutex.Unlock()
			}

			return true
		})

		for _, instanceId := range toDelete {
			log.Debugf("removing %s instance from expected hearbeats map", instanceId)
			heartbeatsMap.Delete(instanceId)
		}
		timer.Reset(api.HeartbeatCheckerTimeout * time.Second)
	}
}

func cleanUnresponsiveInstance(serviceId, instanceId string, alive <-chan struct{}) {
	timer := time.NewTimer(initTimeout * time.Second)

	select {
	case <-alive:
		log.Debugf("instance %s is up", instanceId)
		sendServicesTable()
		return
	case <-timer.C:
		_, ok := servicesTable.GetService(serviceId)
		if !ok {
			log.Debugf("service %s was removed meanwhile, ignoring", serviceId)
			return
		}

		log.Debugf("instance %s never reported, deleting it from service %s", instanceId, serviceId)
		removeInstance(serviceId, instanceId)

		log.Debugf("warning scheduler to remove instance %s", instanceId)
		instancePath := scheduler.GetInstancePath(instanceId)
		req := http_utils.BuildRequest(http.MethodDelete, scheduler.DefaultHostPort, instancePath, nil)
		status, _ := http_utils.DoRequest(httpClient, req, nil)
		if status != http.StatusOK {
			log.Warnf("while trying to remove instance %s after timeout, scheduler returned status %d",
				instanceId, status)
		}

		if len(servicesTable.GetAllServiceInstances(serviceId)) == 0 {
			servicesTable.DeleteService(serviceId)
		}
	}
}

func resolveInstance(originalHost string, originalPort nat.Port, instance *api.Instance) (*api.ResolvedDTO, bool) {
	if instance.Local {
		return &api.ResolvedDTO{
			Host: originalHost,
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
