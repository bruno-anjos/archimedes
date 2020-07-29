package main

import (
	"fmt"
	"net/http"

	"github.com/bruno-anjos/solution-utils/http_utils"
)

// Route names
const (
	registerServiceName          = "REGISTER_SERVICE"
	deleteServiceInstanceName    = "DELETE_SERVICE_INSTANCE"
	heartbeatServiceInstanceName = "HEARTBEAT_SERVICE_INSTANCE"
	getAllServicesName           = "GET_ALL_SERVICES"
	getAllServiceInstancesName   = "GET_ALL_SERVICE_INSTANCES"
	getAllInstancesName          = "GET_ALL_INSTANCES"
	getServiceInstanceName       = "GET_SERVICE_INSTANCE"
	getInstanceName              = "GET_INSTANCE"
	changeInstanceStateName      = "CHANGE_INSTANCE_STATE"
)

// Paths
const (
	PrefixPath = "/archimedes"

	ServicesPath        = "/services"
	ServicePath         = "/services/%s"
	ServiceInstancePath = "/services/%s/%s"

	InstancesPath = "/instances"
	InstancePath  = "/instances/%s"
)

// Path variables
const (
	serviceIdPathVar  = "serviceId"
	instanceIdPathVar = "instanceId"
	statusQueryVar    = "status"
)

var (
	_serviceIdPathVarFormatted  = fmt.Sprintf(http_utils.PathVarFormat, serviceIdPathVar)
	_instanceIdPathVarFormatted = fmt.Sprintf(http_utils.PathVarFormat, instanceIdPathVar)

	servicesRoute        = ServicesPath
	serviceRoute         = fmt.Sprintf(ServicePath, _serviceIdPathVarFormatted)
	serviceInstanceRoute = fmt.Sprintf(ServiceInstancePath, _serviceIdPathVarFormatted, _instanceIdPathVarFormatted)
	instancesRoute       = InstancesPath
	instanceRoute        = fmt.Sprintf(InstancePath, _instanceIdPathVarFormatted)
)

var routes = []http_utils.Route{
	{
		Name:        changeInstanceStateName,
		Method:      http.MethodPut,
		Pattern:     serviceInstanceRoute,
		QueryParams: []string{statusQueryVar, fmt.Sprintf(http_utils.PathVarFormat, statusQueryVar)},
		HandlerFunc: changeInstanceStateHandler,
	},

	{
		Name:        registerServiceName,
		Method:      http.MethodPost,
		Pattern:     serviceRoute,
		HandlerFunc: registerServiceHandler,
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
		Name:        getAllInstancesName,
		Method:      http.MethodGet,
		Pattern:     instancesRoute,
		HandlerFunc: getAllInstancesHandler,
	},

	{
		Name:        getServiceInstanceName,
		Method:      http.MethodGet,
		Pattern:     serviceInstanceRoute,
		HandlerFunc: getServiceInstanceHandler,
	},

	{
		Name:        getInstanceName,
		Method:      http.MethodGet,
		Pattern:     instanceRoute,
		HandlerFunc: getInstanceHandler,
	},
}
