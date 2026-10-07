//go:build wasip1

package main

import (
	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-baota/backend"
)

var runtime = pluginbridge.NewGuestRuntime(backend.NewGuestHandler(pluginbridge.Default().ConfidentialOutbound()))

//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 { return runtime.Alloc(size) }

//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	p, n, err := runtime.Execute(length)
	if err != nil {
		return 0
	}
	return uint64(p)<<32 | uint64(n)
}

//go:wasmexport lina_host_call_alloc
func HostCallAlloc(size uint32) uint32 { return runtime.HostCallAlloc(size) }
func main()                            {}
