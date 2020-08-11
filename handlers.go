package main

import (
	"crypto/sha256"
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
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type (
	typeLocalServicesMapKey   = string
	typeLocalServicesMapValue = *api.Service

	typeHeartbeatsMapKey   = string
	typeHeartbeatsMapValue = *api.PairServiceIdStatus

	typeInitChansMapValue = chan struct{}

	typeNeighborsMapKey   = string
	typeNeighborsMapValue = *Neighbor

	Neighbor struct {
		ArchimedesId string
		Addr         string
	}

	typeIgnoreMapValue = *IgnoreMapEntry

	IgnoreMapEntry struct {
		IgnoreAddrs *sync.Map
	}
)

const (
	// in seconds
	httpClientTimeout        = 20
	initTimeout              = 60
	sendServicesKnownTimeout = 20
	maxHops                  = 2
)

var (
	httpClient *http.Client

	localServicesMap sync.Map
	instancesMap     sync.Map
	heartbeatsMap    sync.Map
	initChansMap     sync.Map

	neighbors sync.Map

	ignoreMap     sync.Map
	servicesTable *ServicesTable

	archimedesId string
)

func init() {
	httpClient = &http.Client{
		Timeout: httpClientTimeout * time.Second,
	}

	localServicesMap = sync.Map{}
	heartbeatsMap = sync.Map{}
	instancesMap = sync.Map{}
	initChansMap = sync.Map{}

	neighbors = sync.Map{}

	ignoreMap = sync.Map{}
	servicesTable = NewServicesTable()

	archimedesId = uuid.New().String()

	log.Infof("ARCHIMEDES ID: %s", archimedesId)

	go instanceHeartbeatChecker()
	go sendKnownServicesPeriodically()
}

func sendKnownServicesPeriodically() {
	timer := time.NewTimer(sendServicesKnownTimeout * time.Second)

	for {
		<-timer.C

		discoverMsg := buildDiscoverMsg()

		neighbors.Range(func(key, value interface{}) bool {
			neighbor := value.(typeNeighborsMapValue)

			req := http_utils.BuildRequest(http.MethodPost, neighbor.Addr, api.GetDiscoverPath(), discoverMsg)
			status, _ := http_utils.DoRequest(httpClient, req, nil)

			if status != http.StatusOK {
				log.Fatalf("got status %d", status)
			}

			return true
		})

		timer.Reset(sendServicesKnownTimeout * time.Second)
	}
}

func buildDiscoverMsg() *api.DiscoverDTO {
	var (
		serviceIds    []string
		services      []*api.ServiceDTO
		instances     map[string][]string
		instancesDTOs map[string]*api.InstanceDTO
		h             = sha256.New()
	)

	instances = map[string][]string{}
	instancesDTOs = map[string]*api.InstanceDTO{}

	localServicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeLocalServicesMapKey)
		service := value.(typeLocalServicesMapValue)
		serviceDTO := &api.ServiceDTO{
			Ports: service.Ports,
		}

		serviceIds = append(serviceIds, serviceId)
		services = append(services, serviceDTO)

		h.Write([]byte(serviceId))

		service.InstancesMap.Range(func(key, value interface{}) bool {
			instance := value.(api.TypeInstancesMapValue)
			instanceDTO := &api.InstanceDTO{
				Static:          false,
				PortTranslation: instance.PortTranslation,
				Local:           false,
			}

			instances[serviceId] = append(instances[serviceId], instance.Id)
			instancesDTOs[instance.Id] = instanceDTO

			h.Write([]byte(instance.Id))

			return true
		})

		return true
	})

	return &api.DiscoverDTO{
		MessageId:     uuid.New(),
		Host:          archimedesId,
		ServicesIds:   serviceIds,
		Services:      services,
		Instances:     instances,
		InstancesDTOs: instancesDTOs,
		ServicesHash:  h.Sum(nil),
		Hops:          0,
		MaxHops:       maxHops,
		Timestamp:     time.Now().Format(time.RFC3339),
	}
}

func addNeighborHandler(w http.ResponseWriter, r *http.Request) {
	neighbor := &Neighbor{}
	err := json.NewDecoder(r.Body).Decode(neighbor)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err)
		panic(err)
	}

	if neighbor.ArchimedesId == "" {
		panic("empty archimedes id")
	}

	log.Debugf("added neighbor %s in %s", neighbor.ArchimedesId, neighbor.Addr)

	neighbors.Store(neighbor.ArchimedesId, neighbor)
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

	log.Debugf("received services from %s", r.RemoteAddr)

	// increase hops on reception
	discoverDTO.Hops++

	servicesTable.UpdateTableWithDiscoverMessage(discoverDTO.Host, &discoverDTO)

	newMap := sync.Map{}
	newMap.Store(discoverDTO.Host, struct{}{})
	entry := &IgnoreMapEntry{IgnoreAddrs: &newMap}
	value, loaded := ignoreMap.LoadOrStore(discoverDTO.MessageId, entry)
	if loaded {
		entry = value.(typeIgnoreMapValue)
		entry.IgnoreAddrs.Store(discoverDTO.MessageId, struct{}{})
	} else {
		go propagateMessageAsync(discoverDTO.Host, discoverDTO)
	}
}

func propagateMessageAsync(neighborSent string, discover api.DiscoverDTO) {
	randInt := rand.Intn(500)
	time.Sleep(time.Duration(randInt) * time.Millisecond)

	if discover.Hops+1 > discover.MaxHops {
		return
	}

	neighbors.Range(func(key, value interface{}) bool {
		neighborId := key.(typeNeighborsMapKey)

		if neighborId == discover.Host {
			log.Debugf("not propagating message %s due to %s being the host", discover.MessageId, neighborId)
			return true
		}

		if neighborId == neighborSent {
			log.Debugf("not propagating message %s due to %s being the neighbor that i received from",
				discover.MessageId, neighborId)
			return true
		}

		ignoreValue, ok := ignoreMap.Load(discover.MessageId)
		if ok {
			ignoreAddrs := ignoreValue.(typeIgnoreMapValue).IgnoreAddrs
			_, ok = ignoreAddrs.Load(neighborId)
			if ok {
				log.Debugf("not propagating message %s to %s due to being in ignore map", discover.MessageId,
					neighborId)
				return true
			}
		}

		neighbor := value.(typeNeighborsMapValue)

		log.Debugf("propagating msg from %s to %s", neighborSent, neighborId)

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
				serviceValue, ok := localServicesMap.Load(pairServiceStatus.ServiceId)
				// case where the instance will be removed and there is a service for that instance
				if ok {
					service := serviceValue.(typeLocalServicesMapValue)
					service.InstancesMap.Delete(instanceId)
					instancesMap.Delete(instanceId)
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
		return
	case <-timer.C:
		value, ok := localServicesMap.Load(serviceId)
		if !ok {
			log.Debugf("service %s was removed meanwhile, ignoring", serviceId)
			return
		}

		log.Debugf("instance %s never reported, deleting it from service %s", instanceId, serviceId)
		service := value.(typeLocalServicesMapValue)
		removeInstance(service, instanceId)

		log.Debugf("warning scheduler to remove instance %s", instanceId)
		instancePath := scheduler.GetInstancePath(instanceId)
		req := http_utils.BuildRequest(http.MethodDelete, scheduler.DefaultHostPort, instancePath, nil)
		status, _ := http_utils.DoRequest(httpClient, req, nil)
		if status != http.StatusOK {
			log.Warnf("while trying to remove instance %s after timeout, scheduler returned status %d",
				instanceId, status)
		}
	}
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
		Id:           serviceId,
		Ports:        serviceDTO.Ports,
		InstancesMap: &sync.Map{},
	}

	_, loaded := localServicesMap.LoadOrStore(serviceId, service)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
	}

	log.Debugf("added service %s", serviceId)
}

func deleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteService handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)
	_, ok := localServicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	localServicesMap.Delete(serviceId)

	log.Debugf("deleted service %s", serviceId)
}

func registerServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	value, ok := localServicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeLocalServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	instanceDTO := api.InstanceDTO{}
	err := json.NewDecoder(r.Body).Decode(&instanceDTO)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
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
		Service:         service,
		Ip:              host,
		PortTranslation: instanceDTO.PortTranslation,
		Initialized:     instanceDTO.Static,
		Local:           instanceDTO.Local,
	}

	initChansMap.Store(instanceId, initChan)

	_, loaded := service.InstancesMap.LoadOrStore(instanceId, instance)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
	}

	instancesMap.Store(instanceId, instance)

	if !instanceDTO.Static {
		go cleanUnresponsiveInstance(serviceId, instanceId, initChan)
		log.Debugf("added INTERACTIVE instance %s to instances waiting for heartbeat", instanceId)
	} else {
		log.Debugf("added STATIC instance %s", instanceId)
	}

}

func deleteServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)
	value, ok := localServicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeLocalServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)
	_, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	removeInstance(service, instanceId)

	log.Debugf("deleted instance %s from service %s", instanceId, serviceId)
}

func heartbeatServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in heartbeatService handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	value, ok := localServicesMap.Load(serviceId)
	if !ok {
		log.Warnf("ignoring heartbeat since service %s wasn't registered", serviceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeLocalServicesMapValue)
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	value, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		log.Warnf("ignoring heartbeat from instance %s since it wasn't registered", instanceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instance := value.(api.TypeInstancesMapValue)
	if !instance.Initialized {
		value, ok = initChansMap.Load(instance.Id)
		if !ok {
			log.Warnf("ignoring heartbeat from instance %s since it didnt have an init channel", instanceId)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		instance.Initialized = true
		initChan := value.(typeInitChansMapValue)
		close(initChan)
	}

	value, ok = heartbeatsMap.Load(instanceId)
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

	services := map[string]*api.ServiceDTO{}

	localServicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeLocalServicesMapKey)
		service := value.(typeLocalServicesMapValue)
		services[serviceId] = &api.ServiceDTO{
			Ports: service.Ports,
		}
		return true
	})

	http_utils.SendJSONReplyOK(w, services)
}

func getAllServiceInstancesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllServiceInstances handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	value, ok := localServicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeLocalServicesMapValue)
	instances := map[api.TypeInstancesMapKey]api.TypeInstancesMapValue{}
	var instanceIds []api.TypeInstancesMapKey

	service.InstancesMap.Range(func(key, value interface{}) bool {
		instanceId := key.(api.TypeInstancesMapKey)
		instance := value.(api.TypeInstancesMapValue)

		instances[instanceId] = instance
		instanceIds = append(instanceIds, instanceId)
		return true
	})

	response := api.CompletedServiceDTO{
		Ports:        service.Ports,
		InstancesIds: instanceIds,
		InstancesMap: instances,
	}

	http_utils.SendJSONReplyOK(w, response)
}

func getServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, ServiceIdPathVar)

	value, ok := localServicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeLocalServicesMapValue)
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	value, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http_utils.SendJSONReplyOK(w, value.(api.TypeInstancesMapValue))
}

func getInstanceHandler(w http.ResponseWriter, r *http.Request) {
	instanceId := http_utils.ExtractPathVar(r, InstanceIdPathVar)

	value, ok := instancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instance := value.(api.TypeInstancesMapValue)
	resp := api.CompletedInstanceDTO{
		Ports:    instance.Service.Ports,
		Instance: instance,
	}

	http_utils.SendJSONReplyOK(w, resp)
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

func removeInstance(service *api.Service, instanceId string) {
	service.InstancesMap.Delete(instanceId)
	instancesMap.Delete(instanceId)
	heartbeatsMap.Delete(instanceId)
}
