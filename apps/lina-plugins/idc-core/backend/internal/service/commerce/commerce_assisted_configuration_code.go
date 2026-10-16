// This file defines the recovery hint for historical quotes without input evidence.
package commerce

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeAssistedConfigurationMissing rejects historical requests without configuration intent.
var CodeAssistedConfigurationMissing = bizerr.MustDefine(
	"IDC_COMMERCE_ASSISTED_QUOTE_CONFIGURATION_MISSING",
	"This quote has no saved product configuration; create a new assisted quote",
	gcode.CodeInvalidOperation,
)
