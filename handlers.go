package main

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bruno-anjos/archimedes/api"
	scheduler "github.com/bruno-anjos/scheduler/api"
	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type (
	typeServicesMapKey   = string
	typeServicesMapValue = *api.Service

	typeHeartbeatsMapKey   = string
	typeHeartbeatsMapValue = *api.PairServiceIdStatus
)

const (
	// in seconds
	httpClientTimeout       = 20
	heartbeatCheckerTimeout = 10
	initTimeout             = 20
)

var (
	httpClient *http.Client

	servicesMap   sync.Map
	instancesMap  sync.Map
	heartbeatsMap sync.Map
)

func init() {
	httpClient = &http.Client{
		Timeout: httpClientTimeout * time.Second,
	}

	servicesMap = sync.Map{}
	heartbeatsMap = sync.Map{}
	instancesMap = sync.Map{}

	go instanceHeartbeatChecker()
}

func instanceHeartbeatChecker() {
	ticker := time.NewTicker(heartbeatCheckerTimeout * time.Second)

	var toDelete []string
	for {
		toDelete = []string{}
		select {
		case <-ticker.C:
			log.Debug("checking heartbeats")
			heartbeatsMap.Range(func(key, value interface{}) bool {
				instanceId := key.(typeHeartbeatsMapKey)
				pairServiceStatus := value.(typeHeartbeatsMapValue)
				pairServiceStatus.Mutex.Lock()

				// case where instance didnt set online status since last status reset, so it has to be removed
				if !pairServiceStatus.IsUp {
					pairServiceStatus.Mutex.Unlock()
					serviceValue, ok := servicesMap.Load(pairServiceStatus.ServiceId)
					// case where the instance will be removed and there is a service for that instance
					if ok {
						service := serviceValue.(typeServicesMapValue)
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
		}

		for _, instanceId := range toDelete {
			log.Debugf("removing %s instance from expected hearbeats map", instanceId)
			heartbeatsMap.Delete(instanceId)
		}
	}
}

func cleanUnresponsiveInstance(serviceId, instanceId string, alive <-chan struct{}) {
	timer := time.NewTimer(initTimeout * time.Second)

	select {
	case <-alive:
		log.Debugf("instance %s is up", instanceId)
		return
	case <-timer.C:
		value, ok := servicesMap.Load(serviceId)
		if !ok {
			log.Debugf("service %s was removed meanwhile, ignoring", serviceId)
			return
		}

		log.Debugf("instance %s never reported, deleting it from service %s", instanceId, serviceId)
		service := value.(typeServicesMapValue)
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

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	serviceDTO := api.ServiceDTO{}
	err := json.NewDecoder(r.Body).Decode(&serviceDTO)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	service := &api.Service{
		Id:           serviceId,
		Dependencies: serviceDTO.Dependencies,
		Ports:        serviceDTO.Ports,
		InstancesMap: &sync.Map{},
	}

	_, loaded := servicesMap.LoadOrStore(serviceId, service)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
	}

	log.Debugf("added service %s", serviceId)
}

func deleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteService handler")

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)
	_, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	servicesMap.Delete(serviceId)

	log.Debugf("deleted service %s", serviceId)
}

func registerServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	value, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)

	instanceDTO := api.InstanceDTO{}
	err := json.NewDecoder(r.Body).Decode(&instanceDTO)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		panic(err)
	}

	initChan := make(chan struct{})
	instance := &api.Instance{
		Id:              instanceId,
		Service:         service,
		Ip:              host,
		PortTranslation: instanceDTO.PortTranslation,
		Initialized:     instanceDTO.Static,
		InitChan:        initChan,
	}

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

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)
	value, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)
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

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	value, ok := servicesMap.Load(serviceId)
	if !ok {
		log.Warnf("ignoring heartbeat since service %s wasn't registered", serviceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeServicesMapValue)
	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)

	value, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		log.Warnf("ignoring heartbeat from instance %s since it wasn't registered", instanceId)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instance := value.(api.TypeInstancesMapValue)
	if !instance.Initialized {
		instance.Initialized = true
		close(instance.InitChan)
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
	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)
	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)

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

	servicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeServicesMapKey)
		service := value.(typeServicesMapValue)
		services[serviceId] = &api.ServiceDTO{
			Dependencies: service.Dependencies,
			Ports:        service.Ports,
		}
		return true
	})

	http_utils.SendJSONReplyOK(w, services)
}

func getAllServiceInstancesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllServiceInstances handler")

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	value, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeServicesMapValue)
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

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	value, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	service := value.(typeServicesMapValue)
	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)

	value, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http_utils.SendJSONReplyOK(w, value.(api.TypeInstancesMapValue))
}

func getInstanceHandler(w http.ResponseWriter, r *http.Request) {
	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)

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
