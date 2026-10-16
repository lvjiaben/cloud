// Package backend exposes the independent template guest entry point.
package backend

import "lina-plugin-website-html/backend/internal/descriptor"

// Handle forwards a public bridge envelope to the read-only descriptor validator.
func Handle(data []byte) []byte {
	return descriptor.Handle(data, "website-html", "idc-portal", "website-template")
}
