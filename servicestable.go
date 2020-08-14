package main

import (
	"sync"

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

func (se *ServicesTableEntry) ToDTO() *api.ServicesTableEntryDTO {
	instances := map[string]*api.Instance{}

	se.EntryLock.RLock()
	defer se.EntryLock.RUnlock()

	se.Instances.Range(func(key, value interface{}) bool {
		instanceId := key.(typeInstancesMapKey)
		instance := value.(typeInstancesMapValue)

		instanceCopy := *instance
		instanceCopy.Local = false
		instances[instanceId] = &instanceCopy

		return true
	})

	return &api.ServicesTableEntryDTO{
		Host:         se.Host.ArchimedesId,
		HostAddr:     se.Host.Addr,
		Service:      se.Service,
		Instances:    instances,
		NumberOfHops: se.NumberOfHops,
		MaxHops:      se.MaxHops,
		Version:      se.Version,
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

func (st *ServicesTable) UpdateService(serviceId string, newEntry *api.ServicesTableEntryDTO) bool {
	value, ok := st.servicesMap.Load(serviceId)
	if !ok {
		log.Fatalf("service %s doesnt exist", serviceId)
	}

	entry := value.(typeServicesTableMapValue)
	entry.EntryLock.RLock()

	log.Debugf("got service on version %d, have %d", entry.Version, newEntry.Version)

	// ignore messages with no new information
	if newEntry.Version <= entry.Version {
		log.Debug("discarding message due to version being older or equal")
		return false
	}

	entry.EntryLock.RUnlock()
	entry.EntryLock.Lock()
	defer entry.EntryLock.Unlock()

	// message is fresher, comes from the closest neighbor or closer and it has new information
	entry.Host = &Neighbor{
		ArchimedesId: newEntry.Host,
		Addr:         newEntry.HostAddr,
	}
	entry.Service = newEntry.Service

	entry.Instances.Range(func(key, value interface{}) bool {
		instanceId := key.(typeInstancesMapKey)
		_, ok = newEntry.Instances[instanceId]
		if !ok {
			st.instancesMap.Delete(instanceId)
		}

		return true
	})

	newInstancesMap := &sync.Map{}
	for instanceId, instance := range newEntry.Instances {
		newInstancesMap.Store(instanceId, instance)
		st.instancesMap.Store(instanceId, instance)
	}

	entry.Instances = newInstancesMap
	entry.NumberOfHops = newEntry.NumberOfHops
	entry.Version = newEntry.Version
	entry.MaxHops = maxHops

	log.Debugf("updated service %s table entry to: %+v", serviceId, entry)
	log.Debugf("with instances %+v", newEntry.Instances)

	return true
}

func (st *ServicesTable) AddService(serviceId string, newEntry *api.ServicesTableEntryDTO) (added bool) {
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
		ArchimedesId: newEntry.Host,
		Addr:         newEntry.HostAddr,
	}
	newTableEntry.Service = newEntry.Service

	newInstancesMap := &sync.Map{}
	for instanceId, instance := range newEntry.Instances {
		newInstancesMap.Store(instanceId, instance)
		st.instancesMap.Store(instanceId, instance)
	}

	newTableEntry.Instances = newInstancesMap
	newTableEntry.NumberOfHops = newEntry.NumberOfHops
	newTableEntry.Version = newEntry.Version
	newTableEntry.MaxHops = maxHops

	servicesMap := &sync.Map{}
	servicesMap.Store(serviceId, struct{}{})
	st.neighborsServicesMap.Store(newEntry.Host, servicesMap)

	added = true

	log.Debugf("added new table entry for service %s: %+v", serviceId, newTableEntry)
	log.Debugf("with instances %+v", newEntry.Instances)

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
	entry.EntryLock.Lock()
	defer entry.EntryLock.Unlock()

	entry.Instances.Store(instanceId, instance)
	entry.Service.Version++

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

	for serviceId, entry := range discoverMsg.Entries {
		log.Debugf("%s has service %s", neighbor, serviceId)

		if entry.Host == archimedesId {
			continue
		}

		_, ok := st.servicesMap.Load(serviceId)
		if ok {
			log.Debugf("service %s already existed, updating", serviceId)
			updated := st.UpdateService(serviceId, entry)
			if updated {
				changed = true
			}
			continue
		}

		st.AddService(serviceId, entry)
		changed = true
	}

	return changed
}

func (st *ServicesTable) ToDiscoverMsg(archimedesId string) *api.DiscoverDTO {
	entries := map[string]*api.ServicesTableEntryDTO{}

	st.servicesMap.Range(func(key, value interface{}) bool {
		serviceId := key.(typeServicesTableMapKey)
		entry := value.(typeServicesTableMapValue)

		entry.EntryLock.RLock()

		if entry.NumberOfHops+1 > maxHops {
			return true
		}

		defer entry.EntryLock.RUnlock()

		entryDTO := entry.ToDTO()
		entryDTO.NumberOfHops++

		entries[serviceId] = entryDTO

		return true
	})

	if len(entries) == 0 {
		return nil
	}

	return &api.DiscoverDTO{
		MessageId:    uuid.New(),
		Origin:       archimedesId,
		NeighborSent: archimedesId,
		Entries:      entries,
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
