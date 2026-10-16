// This file defines safe owner-translated reasons for provider product rejection.

package productautomation

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

// codeSupplierCapabilitiesUnconfirmed tells merchants how to proceed safely.
var codeSupplierCapabilitiesUnconfirmed = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_SUPPLIER_CAPABILITIES_UNCONFIRMED",
	"Supplier domain types and quantity limits are unconfirmed; use manual delivery until the supplier documentation is available",
	gcode.CodeInvalidOperation,
)

// codeCustomerOptionPricesRequired prevents implicitly free purchase options.
var codeCustomerOptionPricesRequired = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_CUSTOMER_OPTION_PRICES_REQUIRED",
	"Set an explicit price for each enabled purchase option in every sale currency and billing period, including zero for free options",
	gcode.CodeInvalidOperation,
)
