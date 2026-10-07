// This file defines stable, supplier-secret-free errors for catalog reads.
// The guest boundary retains its existing safe failure projection.

package v10

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// codeCatalogInvalid rejects malformed selection or pagination input.
	codeCatalogInvalid = bizerr.MustDefine("IDC_V10_CATALOG_INVALID", "The upstream product selection is invalid", gcode.CodeInvalidParameter)
	// codeCatalogUnavailable hides failed, oversized or inconsistent supplier data.
	codeCatalogUnavailable = bizerr.MustDefine("IDC_V10_CATALOG_UNAVAILABLE", "The upstream product catalog could not be read", gcode.CodeInternalError)
)
