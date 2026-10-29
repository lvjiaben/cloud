// This file defines actionable sale-admission errors without changing the
// underlying pricing, copy, tax or automation readiness requirements.

package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	// CodeSaleCopyRequired points operators to the missing bilingual product copy.
	CodeSaleCopyRequired = bizerr.MustDefine("IDC_CATALOG_SALE_COPY_REQUIRED", "Complete both product names and descriptions under Product details before putting this product on sale", gcode.CodeInvalidOperation)
	// CodeSalePricingRequired points operators to missing or incomplete active prices.
	CodeSalePricingRequired = bizerr.MustDefine("IDC_CATALOG_SALE_PRICING_REQUIRED", "Complete active billing periods and prices under Billing & prices before putting this product on sale", gcode.CodeInvalidOperation)
	// CodeSaleTaxRequired requires an explicit merchant tax choice for every currency.
	CodeSaleTaxRequired = bizerr.MustDefine("IDC_CATALOG_SALE_TAX_REQUIRED", "Set a tax rule for every priced currency under Billing & prices before putting this product on sale", gcode.CodeInvalidOperation)
)
