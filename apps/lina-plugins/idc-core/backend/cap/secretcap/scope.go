// This file defines the authenticated in-process scope bound to secret accessors.

package secretcap

const (
	// CoreInvoiceProfileBankAccountScope is the exact owner-only lifecycle
	// scope used for customer invoice-profile bank accounts.
	CoreInvoiceProfileBankAccountScope = "core.invoice-profile-bank-account"
	// CoreSupportUpstreamChannelScope stores the independent legacy-finance
	// upstream ticket channel credential owned by the support domain.
	CoreSupportUpstreamChannelScope = "core.support-upstream-channel"
	// CoreProductAutomationInterfaceScope stores provider credentials declared
	// by one product-fulfillment interface configuration schema.
	CoreProductAutomationInterfaceScope = "core.product-automation-interface"
)

// Scope binds one secret to an installation tenant, owner plugin, and exact
// source-consumer role. Scope is trusted startup composition data, never an
// HTTP request DTO or client-supplied value.
type Scope struct {
	TenantID      int
	OwnerPluginID string
	CallerScope   string
}
