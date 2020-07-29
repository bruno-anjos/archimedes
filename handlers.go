package main

import (
	"net/http"

	"github.com/bruno-anjos/solution-utils/http_utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func registerServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in registerService handler")
	// TODO
	panic("implement me")
}

func deleteServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in deleteService handler")
	// TODO
	panic("implement me")
}

func heartbeatServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in heartbeatService handler")
	// TODO
	panic("implement me")
}

func getAllServicesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllServices handler")
	// TODO
	panic("implement me")
}

func getAllServiceInstancesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllServiceInstances handler")
	// TODO
	panic("implement me")
}

func getAllInstancesHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getAllInstances handler")
	// TODO
	panic("implement me")
}

func getServiceInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getServiceInstance handler")
	// TODO
	panic("implement me")
}

func getInstanceHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("handling request in getInstance handler")
	// TODO
	panic("implement me")
}

const (
	StatusOutOfService = "OUT_OF_SERVICE"
	StatusUp           = "UP"

	errorInvalidStatus = "invalid status query param"
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
		http_utils.SendJSONReplyWithStatus(w, http.StatusBadRequest, errorInvalidStatus)
	}

	// TODO
	panic("implement me")
}
