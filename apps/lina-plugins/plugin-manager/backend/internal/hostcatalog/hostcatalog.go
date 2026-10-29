// Package hostcatalog reads only the host's public authorized HTTP list contract.
// Ordinary GET subrequests repeat authentication, tenant, and permission middleware.
package hostcatalog

// Reader holds no host state, token, cache, or independently created runtime service.
type Reader struct{}
