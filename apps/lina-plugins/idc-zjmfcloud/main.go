//go:build wasip1

// This file exports the WASI guest ABI for the managed ZJMFCloud connector.

package main

import (
	"lina-core/pkg/plugin/pluginbridge"
	resourcebridge "lina-plugin-idc-resource/backend/cap/resourcecap/bridge"
	"lina-plugin-idc-zjmfcloud/backend"
)

var guestRuntime = newGuestRuntime()

func newGuestRuntime() pluginbridge.GuestRuntime {
	servers, err := resourcebridge.NewGuestClient()
	if err != nil {
		return pluginbridge.NewGuestRuntime(backend.NewGuestHandler(nil, nil))
	}
	return pluginbridge.NewGuestRuntime(
		backend.NewGuestHandler(pluginbridge.Default().ConfidentialOutbound(), servers),
	)
}

//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 {
	return guestRuntime.Alloc(size)
}

//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	pointer, responseLength, err := guestRuntime.Execute(length)
	if err != nil {
		return 0
	}
	return uint64(pointer)<<32 | uint64(responseLength)
}

//go:wasmexport lina_host_call_alloc
func HostCallAlloc(size uint32) uint32 {
	return guestRuntime.HostCallAlloc(size)
}

func main() {}
