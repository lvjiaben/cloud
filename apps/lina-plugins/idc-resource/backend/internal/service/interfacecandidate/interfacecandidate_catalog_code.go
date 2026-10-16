// This file keeps catalog-source failures stable and localizable without
// returning supplier payloads, credentials or raw transport errors to operators.
package interfacecandidate

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeCatalogProjectionPending = bizerr.MustDefine("IDC_RESOURCE_CATALOG_PROJECTION_PENDING", "The interface is still being synchronized; retry shortly", gcode.CodeInvalidOperation)
	CodeCatalogInvalid           = bizerr.MustDefine("IDC_RESOURCE_CATALOG_INVALID", "The upstream product query is invalid", gcode.CodeInvalidParameter)
	CodeCatalogNotAccessible     = bizerr.MustDefine("IDC_RESOURCE_CATALOG_NOT_ACCESSIBLE", "The upstream product target is not accessible", gcode.CodeNotAuthorized)
	CodeCatalogUnavailable       = bizerr.MustDefine("IDC_RESOURCE_CATALOG_UNAVAILABLE", "Upstream products could not be read; verify the interface and connector support", gcode.CodeInvalidOperation)
)
