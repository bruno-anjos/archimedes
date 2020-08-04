package main

import (
	"fmt"
	"net/http"

	"github.com/bruno-anjos/archimedes/api"
	"github.com/bruno-anjos/solution-utils/http_utils"
)

// Route names
const (
	registerServiceName                  = "REGISTER_SERVICE"
	deleteServiceName                    = "DELETE_SERVICE"
	registerServiceInstanceName          = "REGISTER_SERVICE"
	registerHeartbeatServiceInstanceName = "REGISTER_HEARTBEAT"
	deleteServiceInstanceName            = "DELETE_SERVICE_INSTANCE"
	heartbeatServiceInstanceName         = "HEARTBEAT_SERVICE_INSTANCE"
	getAllServicesName                   = "GET_ALL_SERVICES"
	getAllServiceInstancesName           = "GET_ALL_SERVICE_INSTANCES"
	getServiceInstanceName               = "GET_SERVICE_INSTANCE"
	getInstanceName                      = "GET_INSTANCE"
	changeInstanceStateName              = "CHANGE_INSTANCE_STATE"
)

// Path variables
const (
	ServiceIdPathVar  = "serviceId"
	InstanceIdPathVar = "instanceId"
)

var (
	_serviceIdPathVarFormatted  = fmt.Sprintf(http_utils.PathVarFormat, ServiceIdPathVar)
	_instanceIdPathVarFormatted = fmt.Sprintf(http_utils.PathVarFormat, InstanceIdPathVar)

	servicesRoute        = api.ServicesPath
	serviceRoute         = fmt.Sprintf(api.ServicePath, _serviceIdPathVarFormatted)
	serviceInstanceRoute = fmt.Sprintf(api.ServiceInstancePath, _serviceIdPathVarFormatted,
		_instanceIdPathVarFormatted)
	serviceInstanceAliveRoute = fmt.Sprintf(api.ServiceInstanceAlivePath, _serviceIdPathVarFormatted,
		_instanceIdPathVarFormatted)
	instanceRoute = fmt.Sprintf(api.InstancePath, _instanceIdPathVarFormatted)
)

var routes = []http_utils.Route{
	{
		Name:        changeInstanceStateName,
		Method:      http.MethodPut,
		Pattern:     serviceInstanceRoute,
		QueryParams: []string{api.StatusQueryVar, fmt.Sprintf(http_utils.PathVarFormat, api.StatusQueryVar)},
		HandlerFunc: changeInstanceStateHandler,
	},

	{
		Name:        registerServiceName,
		Method:      http.MethodPost,
		Pattern:     serviceRoute,
		HandlerFunc: registerServiceHandler,
	},

	{
		Name:        deleteServiceName,
		Method:      http.MethodDelete,
		Pattern:     serviceRoute,
		HandlerFunc: deleteServiceHandler,
	},

	{
		Name:        registerServiceInstanceName,
		Method:      http.MethodPost,
		Pattern:     serviceInstanceRoute,
		HandlerFunc: registerServiceInstanceHandler,
	},

	{
		Name:        deleteServiceInstanceName,
		Method:      http.MethodDelete,
		Pattern:     serviceInstanceRoute,
		HandlerFunc: deleteServiceInstanceHandler,
	},

	{
		Name:        heartbeatServiceInstanceName,
		Method:      http.MethodPut,
		Pattern:     serviceInstanceRoute,
		HandlerFunc: heartbeatServiceInstanceHandler,
	},

	{
		Name:        registerHeartbeatServiceInstanceName,
		Method:      http.MethodPost,
		Pattern:     serviceInstanceAliveRoute,
		HandlerFunc: registerHeartbeatServiceInstanceHandler,
	},

	{
		Name:        getAllServicesName,
		Method:      http.MethodGet,
		Pattern:     servicesRoute,
		HandlerFunc: getAllServicesHandler,
	},

	{
		Name:        getAllServiceInstancesName,
		Method:      http.MethodGet,
		Pattern:     serviceRoute,
		HandlerFunc: getAllServiceInstancesHandler,
	},

	{
		Name:        getInstanceName,
		Method:      http.MethodGet,
		Pattern:     instanceRoute,
		HandlerFunc: getInstanceHandler,
	},

	{
		Name:        getServiceInstanceName,
		Method:      http.MethodGet,
		Pattern:     serviceInstanceRoute,
		HandlerFunc: getServiceInstanceHandler,
	},
}
