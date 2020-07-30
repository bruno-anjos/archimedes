package main

import (
	"net/http"
	"sync"
	"time"

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
	// in milliseconds
	heartbeatCheckerTimeout = 3000
)

var (
	servicesMap   = sync.Map{}
	heartbeatsMap = sync.Map{}
)

func init() {
	go instanceHeartbeatChecker()
}

func instanceHeartbeatChecker() {
	ticker := time.NewTicker(time.Duration(heartbeatCheckerTimeout) * time.Millisecond)

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
					}

					toDelete = append(toDelete, instanceId)
				}

				return true
			})
		}

		for instanceId := range toDelete {
			heartbeatsMap.Delete(instanceId)
		}
	}
}

func registerServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerService handler")

	serviceId := http_utils.ExtractPathVar(r, serviceIdPathVar)

	serviceDTO := ServiceDTO{}
	http_utils.DecodeJSONRequestBody(r, &serviceDTO)

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

	instanceDTO := InstanceDTO{}
	http_utils.DecodeJSONRequestBody(r, &instanceDTO)

	instance := &Instance{
		Id:          instanceId,
		Service:     service,
		InstanceDTO: &instanceDTO,
	}

	_, loaded := service.InstancesMap.LoadOrStore(instanceId, instance)
	if loaded {
		w.WriteHeader(http.StatusConflict)
		return
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

	instancesFromService := value.(typeServicesMapValue)

	instanceId := http_utils.ExtractPathVar(r, instanceIdPathVar)
	_, ok = instancesFromService.InstancesMap.Load(instanceId)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	instancesFromService.InstancesMap.Delete(instanceId)
	heartbeatsMap.Delete(instanceId)
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

	_, ok = service.InstancesMap.Load(instanceId)
	if !ok {
		log.Warnf("ignoring heartbeat from instance %s since it wasn't registered", instanceId)
		return
	}

	value, ok = heartbeatsMap.Load(instanceId)
	if !ok {
		pairServiceStatus := &PairServiceIdStatus{
			ServiceId: serviceId,
			IsUp:      true,
			Mutex:     &sync.Mutex{},
		}
		heartbeatsMap.Store(instanceId, pairServiceStatus)
	} else {
		pairServiceStatus := value.(typeHeartbeatsMapValue)
		pairServiceStatus.Mutex.Lock()
		pairServiceStatus.IsUp = true
		pairServiceStatus.Mutex.Unlock()
	}
}

func getAllServicesHandler(w http.ResponseWriter, _ *http.Request) {
	log.Debug("handling request in getAllServices handler")

	var services map[string]*ServiceDTO

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
	var instances map[string]*InstanceDTO

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

const (
	StatusOutOfService = "OUT_OF_SERVICE"
	StatusUp           = "UP"
)

var (
	allowedStatuses = []string{StatusOutOfService, StatusUp}
)

func changeInstanceStateHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in changeInstanceState handler")

	vars := mux.Vars(r)
	status := vars[statusQueryVar]

	log.Debugf("status query param: %s", status)

	valid := false
	for _, allowedStatus := range allowedStatuses {
		if status == allowedStatus {
			valid = true
			break
		}
	}

	if !valid {
		w.WriteHeader(http.StatusBadRequest)
	}

	// TODO
	panic("implement me")
}
