package main

import (
	"bytes"
	"sync"
	"time"

	"github.com/bruno-anjos/archimedes/api"
	log "github.com/sirupsen/logrus"
)

type (
	ServicesTable struct {
		addLock     sync.Mutex
		servicesMap sync.Map
	}

	typeServicesTableMapValue = *ServicesTableEntry

	ServicesTableEntry struct {
		Host         string
		Instances    map[string]*api.InstanceDTO
		Neighbor     string
		NumberOfHops int
		EntryHash    []byte
		Timestamp    time.Time
		Fresh        bool
		EntryLock    *sync.RWMutex
	}
)

func NewServicesTable() *ServicesTable {
	return &ServicesTable{
		addLock:     sync.Mutex{},
		servicesMap: sync.Map{},
	}
}

func (st *ServicesTable) UpdateTableWithDiscoverMessage(neighbor string, discoverMsg *api.DiscoverDTO) {
	log.Debugf("updating table from message %s", discoverMsg.MessageId.String())

	for _, serviceId := range discoverMsg.ServicesIds {
		log.Debugf("%s has service %s", neighbor, serviceId)

		value, ok := st.servicesMap.Load(serviceId)
		if ok {
			log.Debugf("service %s already existed, updating", serviceId)
			tableEntry := value.(typeServicesTableMapValue)
			st.updateService(serviceId, discoverMsg, tableEntry, neighbor)
			return
		}

		st.addLock.Lock()
		value, ok = st.servicesMap.Load(serviceId)
		if !ok {
			entry := &ServicesTableEntry{
				EntryLock: &sync.RWMutex{},
			}
			entry.EntryLock.Lock()
			st.servicesMap.Store(serviceId, entry)
			st.addLock.Unlock()
			st.addService(serviceId, discoverMsg, neighbor)
			entry.EntryLock.Unlock()
		} else {
			st.addLock.Unlock()
			tableEntry := value.(typeServicesTableMapValue)
			st.updateService(serviceId, discoverMsg, tableEntry, neighbor)
		}
	}
}

func (st *ServicesTable) updateService(serviceId string, discoverMsg *api.DiscoverDTO,
	tableEntry *ServicesTableEntry, neighbor string) {
	tableEntry.EntryLock.RLock()

	msgTime, err := time.Parse(time.RFC3339, discoverMsg.Timestamp)
	if err != nil {
		panic(err)
	}

	// ignore old messages
	if !msgTime.After(tableEntry.Timestamp) {
		log.Debugf("msgTime: %s", discoverMsg.Timestamp)
		log.Debugf("timestamp: %s", tableEntry.Timestamp)
		log.Debugf("discarding message due to oldness")
		return
	}

	// ignore messages with more hops then already known
	if discoverMsg.Hops > tableEntry.NumberOfHops {
		log.Debugf("discarding message due to closer node propagating this service previously")
		return
	}

	// ignore messages with no new information
	if bytes.Equal(discoverMsg.ServicesHash, tableEntry.EntryHash) {
		log.Debugf("discarding message due to hash equal to previous message")
		return
	}

	tableEntry.EntryLock.RUnlock()
	tableEntry.EntryLock.Lock()
	defer tableEntry.EntryLock.Unlock()

	// message is fresher, comes from the closest neighbor or closer and it has new information
	tableEntry.Host = discoverMsg.Host

	serviceInstances := discoverMsg.Instances[serviceId]
	for _, instanceId := range serviceInstances {
		tableEntry.Instances[instanceId] = discoverMsg.InstancesDTOs[instanceId]
	}
	tableEntry.Neighbor = neighbor
	tableEntry.NumberOfHops = discoverMsg.Hops
	tableEntry.EntryHash = discoverMsg.ServicesHash
	tableEntry.Timestamp = msgTime
	tableEntry.Fresh = true

	log.Debugf("updated service %s table entry to: %+v", serviceId, tableEntry)
}

func (st *ServicesTable) addService(serviceId string, discoverMsg *api.DiscoverDTO, neighbor string) {
	value, ok := st.servicesMap.Load(serviceId)
	if ok {
		tableEntry := value.(typeServicesTableMapValue)
		tableEntry.Host = discoverMsg.Host

		serviceInstances := discoverMsg.Instances[serviceId]
		tableEntry.Instances = map[string]*api.InstanceDTO{}
		for _, instanceId := range serviceInstances {
			tableEntry.Instances[instanceId] = discoverMsg.InstancesDTOs[instanceId]
		}
		tableEntry.Neighbor = neighbor
		tableEntry.NumberOfHops = discoverMsg.Hops
		tableEntry.EntryHash = discoverMsg.ServicesHash
		tableEntry.Timestamp = time.Now()
		tableEntry.Fresh = true

		log.Debugf("added new table entry for service %s: %+v", serviceId, tableEntry)
	} else {
		panic("service entry should already exist when adding")
	}
}
