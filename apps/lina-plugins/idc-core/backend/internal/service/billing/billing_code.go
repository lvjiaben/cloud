// This file centralizes stable machine-readable billing error codes.

package billing

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_BILLING_LEDGER_INVALID",
		"The billing ledger request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_BILLING_LEDGER_CONFLICT",
		"The billing ledger request conflicts with immutable history",
		gcode.CodeInvalidOperation,
	)
	CodeInsufficientBalance = bizerr.MustDefine(
		"IDC_BILLING_BALANCE_INSUFFICIENT",
		"The available balance is insufficient",
		gcode.CodeInvalidOperation,
	)
	CodeNotFound = bizerr.MustDefine(
		"IDC_BILLING_LEDGER_NOT_FOUND",
		"The requested billing ledger resource was not found",
		gcode.CodeNotFound,
	)
	CodeIntegrity = bizerr.MustDefine(
		"IDC_BILLING_LEDGER_INTEGRITY",
		"The billing ledger projection failed its integrity check",
		gcode.CodeInternalError,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_BILLING_LEDGER_UNAVAILABLE",
		"The billing ledger service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeInvoiceInvalid = bizerr.MustDefine(
		"IDC_BILLING_INVOICE_INVALID",
		"The invoice operation is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeInvoiceConflict = bizerr.MustDefine(
		"IDC_BILLING_INVOICE_CONFLICT",
		"The invoice operation conflicts with its current financial facts",
		gcode.CodeInvalidOperation,
	)
	CodeInvoiceNotFound = bizerr.MustDefine(
		"IDC_BILLING_INVOICE_NOT_FOUND",
		"The requested invoice was not found",
		gcode.CodeNotFound,
	)
	CodeInvoiceIntegrity = bizerr.MustDefine(
		"IDC_BILLING_INVOICE_INTEGRITY",
		"The invoice financial facts are inconsistent",
		gcode.CodeInternalError,
	)
	CodeInvoiceUnavailable = bizerr.MustDefine(
		"IDC_BILLING_INVOICE_UNAVAILABLE",
		"The invoice service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeSettlementInvalid = bizerr.MustDefine(
		"IDC_BILLING_FINANCIAL_SETTLEMENT_INVALID",
		"The financial settlement request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeSettlementNotEligible = bizerr.MustDefine(
		"IDC_BILLING_FINANCIAL_SETTLEMENT_NOT_ELIGIBLE",
		"The order is not eligible for financial settlement",
		gcode.CodeInvalidOperation,
	)
	CodeSettlementIntegrity = bizerr.MustDefine(
		"IDC_BILLING_FINANCIAL_SETTLEMENT_INTEGRITY",
		"The financial settlement evidence is inconsistent",
		gcode.CodeInternalError,
	)
	CodeSettlementUnavailable = bizerr.MustDefine(
		"IDC_BILLING_FINANCIAL_SETTLEMENT_UNAVAILABLE",
		"The financial settlement service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeDirectoryInvalid = bizerr.MustDefine(
		"IDC_BILLING_DIRECTORY_INVALID",
		"The financial directory request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeDirectoryNotAccessible = bizerr.MustDefine(
		"IDC_BILLING_DIRECTORY_NOT_ACCESSIBLE",
		"The requested financial resource is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeDirectoryIntegrity = bizerr.MustDefine(
		"IDC_BILLING_DIRECTORY_INTEGRITY",
		"The financial directory projection is inconsistent",
		gcode.CodeInternalError,
	)
	CodeDirectoryUnavailable = bizerr.MustDefine(
		"IDC_BILLING_DIRECTORY_UNAVAILABLE",
		"The financial directory is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeCreditInvalid = bizerr.MustDefine(
		"IDC_CREDIT_INVALID",
		"The credit request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeCreditConflict = bizerr.MustDefine(
		"IDC_CREDIT_CONFLICT",
		"The credit request conflicts with current credit facts",
		gcode.CodeInvalidOperation,
	)
	CodeCreditNotFound = bizerr.MustDefine(
		"IDC_CREDIT_NOT_FOUND",
		"The requested credit resource was not found",
		gcode.CodeNotFound,
	)
	CodeCreditIntegrity = bizerr.MustDefine(
		"IDC_CREDIT_INTEGRITY",
		"The credit projection failed its integrity check",
		gcode.CodeInternalError,
	)
	CodeCreditUnavailable = bizerr.MustDefine(
		"IDC_CREDIT_UNAVAILABLE",
		"The credit service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeRefundInvalid = bizerr.MustDefine(
		"IDC_REFUND_INVALID",
		"The refund request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeRefundNotAccessible = bizerr.MustDefine(
		"IDC_REFUND_NOT_ACCESSIBLE",
		"The refund request is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeRefundNotEligible = bizerr.MustDefine(
		"IDC_REFUND_NOT_ELIGIBLE",
		"The selected order items are not eligible for this refund",
		gcode.CodeInvalidOperation,
	)
	CodeRefundConflict = bizerr.MustDefine(
		"IDC_REFUND_CONFLICT",
		"The refund request conflicts with current financial or service facts",
		gcode.CodeInvalidOperation,
	)
	CodeRefundIntegrity = bizerr.MustDefine(
		"IDC_REFUND_INTEGRITY",
		"The refund projection failed its integrity check",
		gcode.CodeInternalError,
	)
	CodeRefundUnavailable = bizerr.MustDefine(
		"IDC_REFUND_UNAVAILABLE",
		"The refund service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
