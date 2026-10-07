//go:build wasip1

package main

import (
	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-zjmf-finance/backend"
)

var guestRuntime = pluginbridge.NewGuestRuntime(backend.NewGuestHandler(pluginbridge.Default().ConfidentialOutbound()))

//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 { return guestRuntime.Alloc(size) }

//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	pointer, responseLength, err := guestRuntime.Execute(length)
	if err != nil {
		return 0
	}
	return uint64(pointer)<<32 | uint64(responseLength)
}

//go:wasmexport lina_host_call_alloc
func HostCallAlloc(size uint32) uint32 { return guestRuntime.HostCallAlloc(size) }

func main() {}
