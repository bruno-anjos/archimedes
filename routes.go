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
	registerServiceInstanceName          = "REGISTER_SERVICE_INSTANCE"
	deleteServiceInstanceName            = "DELETE_SERVICE_INSTANCE"
	getAllServicesName                   = "GET_ALL_SERVICES"
	getAllServiceInstancesName           = "GET_ALL_SERVICE_INSTANCES"
	getServiceInstanceName               = "GET_SERVICE_INSTANCE"
	getInstanceName                      = "GET_INSTANCE"
	changeInstanceStateName              = "CHANGE_INSTANCE_STATE"
	discoverName                         = "DISCOVER"
	whoAreYouName                        = "WHO_ARE_YOU"
	getTableName                         = "GET_TABLE"
	resolveName                          = "RESOLVE"
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
	instanceRoute  = fmt.Sprintf(api.InstancePath, _instanceIdPathVarFormatted)
	discoverRoute  = api.DiscoverPath
	whoAreYouRoute = api.WhoAreYouPath
	tableRoute     = api.TablePath
	resolveRoute   = api.ResolvePath
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

	{
		Name:        discoverName,
		Method:      http.MethodPost,
		Pattern:     discoverRoute,
		HandlerFunc: discoverHandler,
	},

	{
		Name:        whoAreYouName,
		Method:      http.MethodGet,
		Pattern:     whoAreYouRoute,
		HandlerFunc: whoAreYouHandler,
	},

	{
		Name:        getTableName,
		Method:      http.MethodGet,
		Pattern:     tableRoute,
		HandlerFunc: getServicesTableHandler,
	},

	{
		Name:        resolveName,
		Method:      http.MethodPost,
		Pattern:     resolveRoute,
		HandlerFunc: resolveHandler,
	},
}
