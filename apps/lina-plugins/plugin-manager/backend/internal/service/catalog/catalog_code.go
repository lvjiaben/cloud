// This file defines structured catalog errors without exposing host internals.

package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	// CodeCategoryInvalid rejects unconfigured tabs instead of returning a misleading empty page.
	CodeCategoryInvalid = bizerr.MustDefine("plugin_manager.category_invalid", "The plugin category is unavailable. Refresh the page.", gcode.CodeInvalidParameter)
	// CodeCatalogIncomplete prevents truncated totals when the bounded host scan cannot complete.
	CodeCatalogIncomplete = bizerr.MustDefine("plugin_manager.catalog_incomplete", "The plugin catalog could not be read completely. Retry or narrow the search.", gcode.CodeOperationFailed)
)
