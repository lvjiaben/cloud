// This file localizes management errors and prevents database failures leaking.
package salesmanagement

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	codeDenied           = bizerr.MustDefine("IDC_SALES_MANAGEMENT_DENIED", "You do not have access to manage these agents or sales records", gcode.CodeNotAuthorized)
	codeInvalid          = bizerr.MustDefine("IDC_SALES_MANAGEMENT_INVALID", "Check the selected customer, tier and entered values", gcode.CodeInvalidParameter)
	codeConflict         = bizerr.MustDefine("IDC_SALES_MANAGEMENT_CONFLICT", "This record changed; refresh and try again", gcode.CodeInvalidOperation)
	codeUnavailable      = bizerr.MustDefine("IDC_SALES_MANAGEMENT_UNAVAILABLE", "Sales management could not be completed; please retry", gcode.CodeInternalError)
	codeRecoveryDue      = bizerr.MustDefine("IDC_SALES_MANAGEMENT_RECOVERY_DUE", "Refunded commission is still owed; wait for future commission to offset it before recording another payment", gcode.CodeInvalidOperation)
	codeStaffSearchBroad = bizerr.MustDefine("IDC_SALES_MANAGEMENT_STAFF_SEARCH_BROAD", "Enter a more specific employee name to narrow the search", gcode.CodeInvalidParameter)
)

func managementError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, codeUnavailable)
}
