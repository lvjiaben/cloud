//go:build wasip1

// This file initializes the managed guest once and refuses every ABI entry
// if startup dependencies cannot be constructed.

package main

import (
	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-peer/backend"
	"os"
)

var guestRuntime, guestRuntimeError = newGuestRuntime()

func newGuestRuntime() (pluginbridge.GuestRuntime, error) {
	handler, err := backend.NewGuestHandler(pluginbridge.Default().ConfidentialOutbound())
	if err != nil {
		return nil, err
	}
	return pluginbridge.NewGuestRuntime(handler), nil
}

//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 {
	if guestRuntimeError != nil {
		return 0
	}
	return guestRuntime.Alloc(size)
}

//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	if guestRuntimeError != nil {
		return 0
	}
	pointer, responseLength, err := guestRuntime.Execute(length)
	if err != nil {
		return 0
	}
	return uint64(pointer)<<32 | uint64(responseLength)
}

//go:wasmexport lina_host_call_alloc
func HostCallAlloc(size uint32) uint32 {
	if guestRuntimeError != nil {
		return 0
	}
	return guestRuntime.HostCallAlloc(size)
}

func main() {
	if guestRuntimeError != nil {
		os.Exit(1)
	}
}
