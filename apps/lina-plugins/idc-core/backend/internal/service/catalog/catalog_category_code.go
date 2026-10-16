// This file defines the catalog category depth rejection shared by create, update and restore.

package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeCategoryDepthExceeded limits product categories to two levels.
var CodeCategoryDepthExceeded = bizerr.MustDefine(
	"IDC_CATALOG_CATEGORY_DEPTH_EXCEEDED",
	"Product categories support at most two levels",
	gcode.CodeInvalidOperation,
)
