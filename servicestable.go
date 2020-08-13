package main

import (
	"sync"
	"time"

	"github.com/bruno-anjos/archimedes/api"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type (
	ServicesTableEntry struct {
		Host         *Neighbor
		Service      *api.Service
		Instances    *sync.Map
		NumberOfHops int
		MaxHops      int
		Version      int
		Timestamp    time.Time
		EntryLock    *sync.RWMutex
	}
)

func NewTempServiceTableEntry() *ServicesTableEntry {
	return &ServicesTableEntry{
		Host:         nil,
		Service:      nil,
		Instances:    nil,
		NumberOfHops: 0,
		MaxHops:      0,
		Version:      0,
		EntryLock:    &sync.RWMutex{},
	}
}

type (
	ServicesTable struct {
		addLock              sync.Mutex
		servicesMap          sync.Map
		instancesMap         sync.Map
		neighborsServicesMap sync.Map
	}

	typeServicesTableMapKey   = string
	typeServicesTableMapValue = *ServicesTableEntry

	typeInstancesMapKey   = string
	typeInstancesMapValue = *api.Instance

	typeNeighborsServicesMapKey   = string
	typeNeighborsServicesMapValue = *sync.Map
)

func NewServicesTable() *ServicesTable {
	return &ServicesTable{
		addLock:              sync.Mutex{},
		servicesMap:          sync.Map{},
		instancesMap:         sync.Map{},
		neighborsServicesMap: sync.Map{},
	}
}

func (st *ServicesTable) UpdateService(serviceId, host, hostAddr string, service *api.Service,
	instances map[string]*api.Instance, numberOfHops, maxHops, version int) bool {

	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		log.Fatalf("service %s doesnt exist", serviceId)
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()

	// ignore messages with no new information
	if version < entry.Version {
		log.Debug("discarding message due to version being older or equal")
		return false
	}

	entry.EntryLock.RUnlock()
	entry.EntryLock.Lock()
	defer entry.EntryLock.Unlock()

	// message is fresher, comes from the closest neighbor or closer and it has new information
	entry.Host = &Neighbor{
		ArchimedesId: host,
		Addr:         hostAddr,
	}
	entry.Service = service

	entry.Instances.Range(func(key, value interface{}) bool {
		instanceId := key.(typeInstancesMapKey)
		_, ok = instances[instanceId]
		if !ok {
			st.instancesMap.Delete(instanceId)
		}

		return true
	})

	newInstancesMap := &sync.Map{}
	for instanceId, instance := range instances {
		newInstancesMap.Store(instanceId, instance)
		st.instancesMap.Store(instanceId, instance)
	}

	entry.Instances = newInstancesMap
	entry.NumberOfHops = numberOfHops
	entry.Version = version
	entry.MaxHops = maxHops

	log.Debugf("updated service %s table entry to: %+v", serviceId, entry)

	return true
}

func (st *ServicesTable) AddService(serviceId string, host, hostAddr string, service *api.Service,
	instances map[string]*api.Instance, numberOfHops, maxHops, version int) (added bool) {

	_, ok := st.servicesMap.Load(serviceId)
	if ok {
		added = false
		return
	}

	st.addLock.Lock()
	_, ok = st.servicesMap.Load(serviceId)
	if ok {
		st.addLock.Unlock()
		added = false
		return
	}

	newTableEntry := NewTempServiceTableEntry()
	newTableEntry.EntryLock.Lock()
	defer newTableEntry.EntryLock.Unlock()
	st.servicesMap.Store(serviceId, newTableEntry)
	st.addLock.Unlock()

	newTableEntry.Host = &Neighbor{
		ArchimedesId: host,
		Addr:         hostAddr,
	}
	newTableEntry.Service = service

	newInstancesMap := &sync.Map{}
	for instanceId, instance := range instances {
		newInstancesMap.Store(instanceId, instance)
		st.instancesMap.Store(instanceId, instance)
	}

	newTableEntry.Instances = newInstancesMap
	newTableEntry.NumberOfHops = numberOfHops
	newTableEntry.MaxHops = maxHops
	newTableEntry.Version = version

	servicesMap := &sync.Map{}
	servicesMap.Store(serviceId, struct{}{})
	st.neighborsServicesMap.Store(host, servicesMap)

	added = true

	log.Debugf("added new table entry for service %s: %+v", serviceId, newTableEntry)

	return
}

func (st *ServicesTable) GetService(serviceId string) (service *api.Service, ok bool) {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		return nil, false
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	return entry.Service, true
}

func (st *ServicesTable) GetAllServices() map[string]*api.Service {
	services := map[string]*api.Service{}

	st.servicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(string)
		entry := value.(typeServicesTableMapValue)
		entry.EntryLock.RLock()
		defer entry.EntryLock.RUnlock()

		services[serviceId] = entry.Service

		return true
	})

	return services
}

func (st *ServicesTable) GetAllServiceInstances(serviceId string) map[string]*api.Instance {
	instances := map[string]*api.Instance{}

	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		return instances
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	entry.Instances.Range(func(key, value interface{}) bool {
		instanceId := key.(string)
		instance := value.(typeInstancesMapValue)

		instances[instanceId] = instance

		return true
	})

	return instances
}

func (st *ServicesTable) AddInstance(serviceId, instanceId string, instance *api.Instance) (added bool) {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		added = false
		return
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	entry.Instances.Store(instanceId, instance)

	st.instancesMap.Store(instanceId, instance)

	added = true
	return
}

func (st *ServicesTable) ServiceHasInstance(serviceId, instanceId string) bool {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		return false
	}

	entry := value.(typeServicesTableMapValue)

	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	_, ok = entry.Instances.Load(instanceId)

	return ok
}

func (st *ServicesTable) GetServiceInstance(serviceId, instanceId string) (*api.Instance, bool) {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		return nil, false
	}

	entry := value.(typeServicesTableMapValue)

	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	value, ok = entry.Instances.Load(instanceId)

	return value.(typeInstancesMapValue), ok
}

func (st *ServicesTable) GetInstance(instanceId string) (instance *api.Instance, ok bool) {
	value, ok := st.instancesMap.Load(instanceId)
	if !ok {
		return nil, false
	}

	return value.(typeInstancesMapValue), true
}

func (st *ServicesTable) DeleteService(serviceId string) {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		return
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()
	defer entry.EntryLock.RUnlock()

	entry.Instances.Range(func(key, _ interface{}) bool {
		instanceId := key.(typeInstancesMapKey)
		st.DeleteInstance(serviceId, instanceId)
		return true
	})

	st.servicesMap.Delete(serviceId)
}

func (st *ServicesTable) DeleteInstance(serviceId, instanceId string) {
	value, ok := st.instancesMap.Load(serviceId)
	if ok {
		entry := value.(typeServicesTableMapValue)
		entry.EntryLock.RLock()
		entry.Instances.Delete(instanceId)
		entry.EntryLock.RUnlock()
	}

	st.instancesMap.Delete(instanceId)
}

func (st *ServicesTable) UpdateTableWithDiscoverMessage(neighbor string, discoverMsg *api.DiscoverDTO) (changed bool) {
	log.Debugf("updating table from message %s", discoverMsg.MessageId.String())

	changed = false

	var (
		host, hostAddr          string
		instances               map[string]*api.Instance
		hops, sMaxHops, version int
	)

	for serviceId, service := range discoverMsg.Services {
		log.Debugf("%s has service %s", neighbor, serviceId)

		if discoverMsg.Host == archimedesId {
			continue
		}

		host = discoverMsg.Host
		hostAddr = discoverMsg.HostAddr
		instances = discoverMsg.Instances
		hops = discoverMsg.Hops
		sMaxHops = discoverMsg.MaxHops
		version = service.Version

		_, ok := st.servicesMap.Load(serviceId)
		if ok {
			log.Debugf("service %s already existed, updating", serviceId)
			updated := st.UpdateService(serviceId, host, hostAddr, service, instances, hops, sMaxHops, version)
			if updated {
				changed = true
			}
		}

		st.AddService(serviceId, host, hostAddr, service, instances, hops, sMaxHops, version)
		changed = true
	}

	return changed
}

func (st *ServicesTable) ToDiscoverMsg(archimedesId string) *api.DiscoverDTO {
	var (
		services           map[string]*api.Service
		serviceToInstances map[string][]string
		instances          map[string]*api.Instance
	)

	services = map[string]*api.Service{}
	serviceToInstances = map[string][]string{}
	instances = map[string]*api.Instance{}

	st.servicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeServicesTableMapKey)
		entry := value.(typeServicesTableMapValue)

		entry.EntryLock.RLock()
		defer entry.EntryLock.RUnlock()

		services[serviceId] = entry.Service
		entry.Instances.Range(func(key, value interface{}) bool {
			instanceId := key.(typeInstancesMapKey)
			instance := value.(typeInstancesMapValue)
			serviceToInstances[serviceId] = append(serviceToInstances[serviceId], instanceId)
			instances[instanceId] = instance
			return true
		})

		return true
	})

	return &api.DiscoverDTO{
		MessageId:          uuid.New(),
		Host:               archimedesId,
		Services:           services,
		ServiceToInstances: serviceToInstances,
		Instances:          instances,
		Hops:               0,
		MaxHops:            maxHops,
	}
}

func (st *ServicesTable) DeleteNeighborServices(neighborId string) {
	value, ok := st.neighborsServicesMap.Load(neighborId)
	if !ok {
		return
	}

	services := value.(typeNeighborsServicesMapValue)
	services.Range(func(key, _ interface{}) bool {
		serviceId := key.(typeNeighborsServicesMapKey)
		servicesTable.DeleteService(serviceId)
		return true
	})
}
