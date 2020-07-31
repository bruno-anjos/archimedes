package main

import (
	"encoding/json"
	"net/http"
	"strings"
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
	typeServicesMapValue = *Service

	typeHeartbeatsMapKey   = string
	typeHeartbeatsMapValue = *PairServiceIdStatus
)

const (
	// in seconds
	httpClientTimeout       = 10
	heartbeatCheckerTimeout = 10
	initTimeout             = 20
)

var (
	httpClient *http.Client

	servicesMap   sync.Map
	heartbeatsMap sync.Map
)

func init() {
	httpClient = &http.Client{
		Timeout: httpClientTimeout * time.Second,
	}

	servicesMap = sync.Map{}
	heartbeatsMap = sync.Map{}

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
		service.InstancesMap.Delete(instanceId)

		log.Debugf("warning scheduler to remove instance %s", instanceId)
		instancePath := scheduler.GetInstancePath(instanceId)
		req := http_utils.BuildRequest(http.MethodDelete, scheduler.DefaultHostPort, instancePath, nil)
		status, _ := http_utils.DoRequest(httpClient, req, nil)
		if status != http.StatusOK {
			log.Warnf("while trying to remove instance %s after timeout, scheduler returned code %d",
				instanceId, status)
		}

		return
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

	service := &Service{
		Id:           serviceId,
		ServiceDTO:   &serviceDTO,
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

	ip := strings.Split(r.RemoteAddr, ",")[0]

	initChan := make(chan struct{})
	instance := &Instance{
		Id:          instanceId,
		Service:     service,
		Ip:          ip,
		InstanceDTO: &instanceDTO,
		Initialized: false,
		InitChan:    initChan,
	}

	_, loaded := service.InstancesMap.LoadOrStore(instanceId, instance)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
	}

	go cleanUnresponsiveInstance(serviceId, instanceId, initChan)

	log.Debugf("added instance %s to instances waiting for heartbeat", instanceId)
}

func deleteServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteServiceInstance handler")

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)
	value, ok := servicesMap.Load(serviceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instancesFromService := value.(typeServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)
	_, ok = instancesFromService.InstancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instancesFromService.InstancesMap.Delete(instanceId)
	heartbeatsMap.Delete(instanceId)

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

	instance := value.(typeInstancesMapValue)
	if !instance.Initialized {
		instance.Initialized = true
		close(instance.InitChan)
	}

	pairServiceStatus := &PairServiceIdStatus{
		ServiceId: serviceId,
		IsUp:      true,
		Mutex:     &sync.Mutex{},
	}
	value, loaded := heartbeatsMap.LoadOrStore(instanceId, pairServiceStatus)
	if loaded {
		pairServiceStatus = value.(typeHeartbeatsMapValue)
		pairServiceStatus.Mutex.Lock()
		pairServiceStatus.IsUp = true
		pairServiceStatus.Mutex.Unlock()
	}

	log.Debugf("got heartbeat from instance %s", instanceId)
}

func getAllServicesHandler(w http.ResponseWriter, _ *http.Request) {
	log.Debug("handling request in getAllServices handler")

	services := map[string]*api.ServiceDTO{}

	servicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeServicesMapKey)
		service := value.(typeServicesMapValue)
		services[serviceId] = service.ServiceDTO
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
	instances := map[string]*api.InstanceDTO{}

	service.InstancesMap.Range(func(key, value interface{}) bool {
		instanceId := key.(typeInstancesMapKey)
		instance := value.(typeInstancesMapValue)
		instances[instanceId] = instance.InstanceDTO
		return true
	})

	http_utils.SendJSONReplyOK(w, instances)
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

	http_utils.SendJSONReplyOK(w, value.(typeInstancesMapValue))
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
