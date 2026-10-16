// This file defines the customer-readable rejection for unsafe trial publication.

package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeTrialFulfillmentUnavailable requires real automatic trial suspension and recovery.
var CodeTrialFulfillmentUnavailable = bizerr.MustDefine(
	"IDC_CATALOG_TRIAL_FULFILLMENT_UNAVAILABLE",
	"Trial products require automatic delivery with pause, resume and formal renewal support; remove the trial plan or choose a supported interface",
	gcode.CodeInvalidOperation,
)
