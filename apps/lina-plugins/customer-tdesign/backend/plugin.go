// Package backend exposes the independent template guest entry point.
package backend

import "lina-plugin-customer-tdesign/backend/internal/descriptor"

// Handle forwards a public bridge envelope to the read-only descriptor validator.
func Handle(data []byte) []byte {
	return descriptor.Handle(data, "customer-tdesign", "idc-portal", "customer-template")
}
