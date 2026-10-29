// Package catalog owns the general product catalog and the explicitly isolated
// legacy server migration bridge used by later FB-52 extraction tasks.
package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

// PermissionCatalogProductPublish is the established permission owned by the
// catalog publication boundary and reused by its production adapters.
const PermissionCatalogProductPublish = "idc:admin:catalog:product:publish"

var (
	// CodeComponentInvalid reports a malformed typed server component value.
	CodeComponentInvalid = bizerr.MustDefine(
		"IDC_CATALOG_COMPONENT_INVALID",
		"The server catalog component value is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeConfigurationInvalid reports an inconsistent fixed or configurable graph.
	CodeConfigurationInvalid = bizerr.MustDefine(
		"IDC_CATALOG_CONFIGURATION_INVALID",
		"The product configuration graph is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeManagementInvalid reports malformed category or product draft input.
	CodeManagementInvalid = bizerr.MustDefine(
		"IDC_CATALOG_MANAGEMENT_INVALID",
		"The catalog management request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeCategoryParentCycle rejects moving a category below itself.
	CodeCategoryParentCycle = bizerr.MustDefine(
		"IDC_CATALOG_CATEGORY_PARENT_CYCLE",
		"A category cannot be moved below itself or one of its descendants",
		gcode.CodeInvalidOperation,
	)
	// CodeProductSlugOccupied reports a merchant-facing URL collision.
	CodeProductSlugOccupied = bizerr.MustDefine(
		"IDC_CATALOG_PRODUCT_SLUG_OCCUPIED",
		"The product URL is already in use",
		gcode.CodeInvalidOperation,
	)
	// CodeManagementNotAccessible hides both authorization and target existence.
	CodeManagementNotAccessible = bizerr.MustDefine(
		"IDC_CATALOG_MANAGEMENT_NOT_ACCESSIBLE",
		"The catalog management target is not accessible",
		gcode.CodeNotAuthorized,
	)
	// CodeManagementConflict reports an incompatible or concurrently changed target.
	CodeManagementConflict = bizerr.MustDefine(
		"IDC_CATALOG_MANAGEMENT_CONFLICT",
		"The catalog management request conflicts with the current state",
		gcode.CodeInvalidOperation,
	)
	// CodeManagementUnavailable hides persistence and audit implementation details.
	CodeManagementUnavailable = bizerr.MustDefine(
		"IDC_CATALOG_MANAGEMENT_UNAVAILABLE",
		"The catalog management service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeFulfillmentPolicyBindingRequired = bizerr.MustDefine(
		"IDC_CATALOG_FULFILLMENT_POLICY_BINDING_REQUIRED",
		"Bind an available automation interface before selecting plugin fulfillment",
		gcode.CodeInvalidOperation,
	)
	CodeFulfillmentPolicyContractChanged = bizerr.MustDefine(
		"IDC_CATALOG_FULFILLMENT_POLICY_CONTRACT_CHANGED",
		"The automation interface changed; bind it again and reselect the fulfillment mode",
		gcode.CodeInvalidOperation,
	)
	CodeFulfillmentPolicyImmutable = bizerr.MustDefine(
		"IDC_CATALOG_FULFILLMENT_POLICY_IMMUTABLE",
		"A published product version cannot change its fulfillment mode",
		gcode.CodeInvalidOperation,
	)
	// CodePublicCatalogInvalid reports malformed anonymous catalog filters or cursors.
	CodePublicCatalogInvalid = bizerr.MustDefine(
		"IDC_PUBLIC_CATALOG_INVALID",
		"The public catalog request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodePublicCatalogNotFound hides unpublished and nonexistent public slugs alike.
	CodePublicCatalogNotFound = bizerr.MustDefine(
		"IDC_PUBLIC_CATALOG_NOT_FOUND",
		"The product is not available",
		gcode.CodeNotFound,
	)
	// CodePublicCatalogUnavailable hides persistence details from anonymous callers.
	CodePublicCatalogUnavailable = bizerr.MustDefine(
		"IDC_PUBLIC_CATALOG_UNAVAILABLE",
		"The product catalog is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodePublicConfigurationInvalid reports malformed or unknown public selections.
	CodePublicConfigurationInvalid = bizerr.MustDefine(
		"IDC_PUBLIC_CONFIGURATION_INVALID",
		"The product configuration selection is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodePublicConfigurationStale requires visitors to reload a changed published graph.
	CodePublicConfigurationStale = bizerr.MustDefine(
		"IDC_PUBLIC_CONFIGURATION_STALE",
		"The product configuration changed; reload the product",
		gcode.CodeInvalidOperation,
	)
	// CodePublicQuoteInvalid reports malformed quote inputs or an invalid complete selection.
	CodePublicQuoteInvalid = bizerr.MustDefine(
		"IDC_PUBLIC_QUOTE_INVALID",
		"The product price request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodePublicQuoteStale requires a fresh published configuration revision.
	CodePublicQuoteStale = bizerr.MustDefine(
		"IDC_PUBLIC_QUOTE_STALE",
		"The product changed; reload it before checking the price",
		gcode.CodeInvalidOperation,
	)
	// CodePublicQuoteNotAvailable rejects incomplete currency, price, or tax facts.
	CodePublicQuoteNotAvailable = bizerr.MustDefine(
		"IDC_PUBLIC_QUOTE_NOT_AVAILABLE",
		"This configuration is not available in the selected currency",
		gcode.CodeInvalidOperation,
	)
	// CodePublicQuoteUnavailable hides local persistence details from anonymous callers.
	CodePublicQuoteUnavailable = bizerr.MustDefine(
		"IDC_PUBLIC_QUOTE_UNAVAILABLE",
		"The product price is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodePublicationNotAccessible prevents publication target disclosure.
	CodePublicationNotAccessible = bizerr.MustDefine(
		"IDC_CATALOG_PUBLICATION_NOT_ACCESSIBLE",
		"The catalog product version is not accessible",
		gcode.CodeNotAuthorized,
	)
	// CodePublicationInvalid reports malformed publication input.
	CodePublicationInvalid = bizerr.MustDefine(
		"IDC_CATALOG_PUBLICATION_INVALID",
		"The catalog product publication request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodePublicationState reports an invalid version lifecycle transition.
	CodePublicationState = bizerr.MustDefine(
		"IDC_CATALOG_PUBLICATION_STATE_INVALID",
		"The catalog product version cannot be published from its current state",
		gcode.CodeInvalidOperation,
	)
	// CodePublicationIncomplete reports a draft that is not safe to sell.
	CodePublicationIncomplete = bizerr.MustDefine(
		"IDC_CATALOG_PUBLICATION_INCOMPLETE",
		"Complete product details, pricing, tax and delivery settings before putting this product on sale",
		gcode.CodeInvalidOperation,
	)
	// CodePublicationUnavailable hides lower-level publication failures.
	CodePublicationUnavailable = bizerr.MustDefine(
		"IDC_CATALOG_PUBLICATION_UNAVAILABLE",
		"Catalog product publication is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodeBillingInvalid reports an inconsistent billing policy or price schedule.
	CodeBillingInvalid = bizerr.MustDefine(
		"IDC_CATALOG_BILLING_INVALID",
		"The catalog billing rule or price schedule is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeBillingAmountOverflow rejects a quote that cannot be represented safely.
	CodeBillingAmountOverflow = bizerr.MustDefine(
		"IDC_CATALOG_BILLING_AMOUNT_OVERFLOW",
		"The catalog billing amount exceeds the supported range",
		gcode.CodeInvalidOperation,
	)
	// CodeCurrencyPricingInvalid rejects malformed explicit-currency pricing input.
	CodeCurrencyPricingInvalid = bizerr.MustDefine(
		"IDC_CATALOG_CURRENCY_PRICING_INVALID",
		"The explicit currency pricing request is invalid",
		gcode.CodeInvalidParameter,
	)
)
