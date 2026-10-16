// This file defines localized errors for retired settlement operations.
package saleschannel

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	codeSettlementRetired       = bizerr.MustDefine("IDC_SALES_SETTLEMENT_RETIRED", "Commission payments are recorded offline by an administrator; settlement applications and payout batches are no longer available", gcode.CodeInvalidOperation)
	codeSettlementReleaseFailed = bizerr.MustDefine("IDC_SALES_SETTLEMENT_RELEASE_FAILED", "Unable to release the historical commission settlement; check its status and evidence", gcode.CodeOperationFailed)
	codeSettlementDenied        = bizerr.MustDefine("IDC_SALES_SETTLEMENT_DENIED", "You do not have access to release this historical commission settlement", gcode.CodeNotAuthorized)
)
