// This file defines safe owner-translated reasons for provider product rejection.

package productautomation

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

// Directory-only options must be captured before removing their source.
var codeOfferOptionsRequired = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_OFFER_OPTIONS_REQUIRED",
	"The existing interface options cannot be preserved yet; refresh or import its sellable options before changing the automation interface",
	gcode.CodeInvalidOperation,
)

// Editing aliases cannot erase an already-frozen directory relationship.
var codeChoiceRelationsRequired = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_CHOICE_RELATIONS_REQUIRED",
	"The option mapping changed but related interface choices could not be confirmed; reload the interface options and retry",
	gcode.CodeInvalidOperation,
)

// Detaching must never discard a protected file/secret policy or a field that
// cannot be represented independently by the current commercial editor.
var codeOfferDetachUnsupported = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_OFFER_DETACH_UNSUPPORTED",
	"This offer contains protected file, secret, imported or unsupported configuration; retain the interface until these fields have an independent delivery definition",
	gcode.CodeInvalidOperation,
)

// An old supplier's parameter identity is never accepted as a new mapping.
var codeOfferMappingRequired = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_OFFER_MAPPING_REQUIRED",
	"The automation interface changed; choose the new parameter for each retained configuration or explicitly keep it as customer information only",
	gcode.CodeInvalidOperation,
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

// codeProviderBusy preserves an authoritative host lock refusal without implying
// that an arbitrary business operation is safe to replay.
var codeProviderBusy = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_PROVIDER_BUSY",
	"The automation connector is busy; retry after its current operation finishes",
	gcode.CodeInvalidOperation,
)

// codeProviderValidationUnavailable identifies a failed pure configuration call.
var codeProviderValidationUnavailable = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_VALIDATION_UNAVAILABLE",
	"The connector cannot validate the product configuration right now; retry or check its availability",
	gcode.CodeOperationFailed,
)

// codeProviderValidationResponseInvalid identifies an incompatible connector
// reply without disclosing its untrusted response body or error text.
var codeProviderValidationResponseInvalid = bizerr.MustDefine(
	"IDC_PRODUCT_AUTOMATION_VALIDATION_RESPONSE_INVALID",
	"The connector returned an incompatible configuration validation result; check the connector version",
	gcode.CodeInvalidOperation,
)
