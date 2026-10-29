// This file defines the public HTTP catalog adapter's structured read error.

package hostcatalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeReadFailed rejects denied, malformed, or oversized public host responses.
var CodeReadFailed = bizerr.MustDefine("plugin_manager.catalog_read_failed", "Unable to read the plugin catalog. Check your permissions and retry.", gcode.CodeOperationFailed)
