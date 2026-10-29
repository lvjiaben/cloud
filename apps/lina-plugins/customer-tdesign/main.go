//go:build wasip1

// This file exposes the public v1 bridge ABI for an independent template guest.
package main

import (
	"lina-plugin-customer-tdesign/backend"
	"unsafe"
)

// Buffers remain rooted for the full synchronous host invocation.
var requestBuffer, responseBuffer []byte

// DynamicRouteAlloc reserves bounded input memory for the next bridge call.
//
//go:wasmexport lina_dynamic_route_alloc
func DynamicRouteAlloc(size uint32) uint32 {
	if size == 0 || size > 1024*1024 {
		return 0
	}
	requestBuffer = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&requestBuffer[0])))
}

// DynamicRouteExecute validates the request and publishes the response pointer/size.
//
//go:wasmexport lina_dynamic_route_execute
func DynamicRouteExecute(length uint32) uint64 {
	if length == 0 || uint64(length) > uint64(len(requestBuffer)) {
		return 0
	}
	responseBuffer = backend.Handle(requestBuffer[:length])
	return uint64(uint32(uintptr(unsafe.Pointer(&responseBuffer[0]))))<<32 | uint64(len(responseBuffer))
}

func main() {}
