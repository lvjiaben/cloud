// This file defines the stable management-permission namespace shared by all IDC plugins.

package idccap

import "strings"

const (
	// DefaultTenantID is the only tenant context accepted by the single-merchant IDC edition.
	DefaultTenantID = 0
	// AdminPermissionPrefix is the mandatory namespace for every IDC management permission.
	AdminPermissionPrefix = "idc:admin:"
	// HostPermissionWildcard is the LinaPro static-route wildcard grant. It is
	// checked explicitly because host authorization capability reads are exact.
	HostPermissionWildcard = "*:*:*"
)

// adminPermissionCatalog is the exhaustive set of concrete IDC management
// permissions owned by the compiled plugin suite.
var adminPermissionCatalog = [...]string{
	"idc:admin:currency:read",
	"idc:admin:currency:write",
	"idc:admin:product-directory:list",
	"idc:admin:product-directory:create",
	"idc:admin:product-directory:update",
	"idc:admin:customer:list",
	"idc:admin:customer:query",
	"idc:admin:audit:list",
	"idc:admin:customer:update",
	"idc:admin:customer:create",
	"idc:admin:customer:assist",
	"idc:admin:customer:tag:batch",
	"idc:admin:customer:contract-policy:write",
	"idc:admin:portal:brand",
	"idc:admin:portal:customer-template",
	"idc:admin:portal:external-menu:view",
	"idc:admin:portal:external-menu:manage",
	"idc:admin:contract-policy:template:list",
	"idc:admin:catalog:product:list",
	"idc:admin:catalog:product:query",
	"idc:admin:catalog:product:publish",
	"idc:admin:catalog:category:list",
	"idc:admin:catalog:category:create",
	"idc:admin:catalog:category:update",
	"idc:admin:catalog:category:delete",
	"idc:admin:catalog:category:restore",
	"idc:admin:catalog:product:create",
	"idc:admin:catalog:product:update",
	"idc:admin:catalog:billing:write",
	"idc:admin:catalog:price:write",
	"idc:admin:catalog:tax:write",
	"idc:admin:catalog:cost:write",
	"idc:admin:catalog:contract-policy:write",
	"idc:admin:catalog:automation-interface:view",
	"idc:admin:catalog:automation-interface:manage",
	"idc:admin:catalog:fulfillment-binding:write",
	"idc:admin:catalog:configuration:write",
	"idc:admin:marketing:overview",
	"idc:admin:marketing:segment:list",
	"idc:admin:marketing:segment:query",
	"idc:admin:marketing:campaign:list",
	"idc:admin:marketing:campaign:query",
	"idc:admin:marketing:asset:list",
	"idc:admin:marketing:asset:query",
	"idc:admin:marketing:points:list",
	"idc:admin:marketing:points:query",
	"idc:admin:marketing:grant:list",
	"idc:admin:marketing:grant:query",
	"idc:admin:marketing:asset:issue",
	"idc:admin:marketing:coupon-code:generate",
	"idc:admin:marketing:asset:revoke",
	"idc:admin:marketing:points:credit",
	"idc:admin:marketing:points:reverse",
	"idc:admin:marketing:grant:post",
	"idc:admin:marketing:grant:reverse",
	"idc:admin:marketing:grant:review",
	"idc:admin:marketing:performance:list",
	"idc:admin:marketing:abuse:list",
	"idc:admin:marketing:abuse:review",
	"idc:admin:marketing:cost:missing:list",
	"idc:admin:marketing:cost:correct",
	"idc:admin:marketing:campaign:newcomer:update",
	"idc:admin:commerce:risk-review:approve",
	"idc:admin:commerce:risk-review:reject",
	"idc:admin:commerce:order:list",
	"idc:admin:commerce:order:query",
	"idc:admin:commerce:contract-policy:write",
	"idc:admin:commerce:assisted-quote:list",
	"idc:admin:commerce:assisted-quote:query",
	"idc:admin:commerce:assisted-quote:create",
	"idc:admin:billing:overview",
	"idc:admin:billing:invoice:list",
	"idc:admin:billing:invoice:query",
	"idc:admin:billing:transaction:list",
	"idc:admin:billing:transaction:query",
	"idc:admin:billing:adjustment:list",
	"idc:admin:billing:adjustment:query",
	"idc:admin:billing:manual-receipt:create",
	"idc:admin:billing:invoice-waiver:create",
	"idc:admin:billing:adjustment:review",
	"idc:admin:billing:adjustment-policy:write",
	"idc:admin:billing:invoice:resend",
	"idc:admin:billing:note:list",
	"idc:admin:billing:note:create",
	"idc:admin:billing:filter:personal",
	"idc:admin:billing:export",
	"idc:admin:billing:credit-application:list",
	"idc:admin:billing:credit-application:query",
	"idc:admin:billing:credit-application:review",
	"idc:admin:billing:credit-account:list",
	"idc:admin:billing:credit-account:query",
	"idc:admin:billing:credit-statement:list",
	"idc:admin:billing:credit-statement:query",
	"idc:admin:billing:credit-policy:write",
	"idc:admin:billing:refund:list",
	"idc:admin:billing:refund:query",
	"idc:admin:billing:refund:review",
	"idc:admin:billing:refund-policy:write",
	"idc:admin:payment:reconciliation:view",
	"idc:admin:payment:reconciliation:supplement-submit",
	"idc:admin:payment:reconciliation:supplement-review",
	"idc:admin:payment:exception-fund:view",
	"idc:admin:payment:exception-fund:allocate",
	"idc:admin:payment:exception-fund:refund",
	"idc:admin:payment:exception-fund:refund-confirm",
	"idc:admin:payment:channel:view",
	"idc:admin:payment:channel:manage",
	"idc:admin:resource:provider-capacity:list",
	"idc:admin:resource:provider-capacity:refresh",
	"idc:admin:resource:configuration:list",
	"idc:admin:resource:configuration:write",
	"idc:admin:resource:overview:list",
	"idc:admin:resource:health:list",
	"idc:admin:resource:health-policy:write",
	"idc:admin:resource:task:list",
	"idc:admin:resource:task:query",
	"idc:admin:resource:task:manual-takeover",
	"idc:admin:resource:server-operation-policy:read",
	"idc:admin:resource:server-operation-policy:write",
	"idc:admin:resource:server-batch:view",
	"idc:admin:resource:server-batch:safe",
	"idc:admin:resource:server-batch:dangerous",
	"idc:admin:resource:accessory-price:read",
	"idc:admin:resource:accessory-price:write",
	"idc:admin:resource:ip-change-policy:read",
	"idc:admin:resource:ip-change-policy:write",
	"idc:admin:resource:server-accessory:view",
	"idc:admin:resource:server:edit",
	"idc:admin:resource:server-accessory:snapshot:create",
	"idc:admin:resource:server-accessory:snapshot:delete",
	"idc:admin:resource:server-accessory:snapshot:restore",
	"idc:admin:resource:server-accessory:backup:create",
	"idc:admin:resource:server-accessory:backup:delete",
	"idc:admin:resource:server-accessory:backup:restore",
	"idc:admin:resource:server-accessory:security-group:attach",
	"idc:admin:resource:server-accessory:security-group:detach",
	"idc:admin:resource:server-accessory:nat:create",
	"idc:admin:resource:server-accessory:nat:delete",
	"idc:admin:resource:server-accessory:virtual-media:mount",
	"idc:admin:resource:server-accessory:virtual-media:unmount",
	"idc:admin:resource:server-accessory:traffic:add",
	"idc:admin:resource:channel-secret:manage",
	"idc:admin:compliance:provider:query",
	"idc:admin:compliance:provider:write",
	"idc:admin:compliance:provider:validate",
	"idc:admin:compliance:supplier:query",
	"idc:admin:compliance:supplier:credential:write",
	"idc:admin:compliance:supplier:subject:bind",
	"idc:admin:compliance:supplier:subject:revoke",
	"idc:admin:compliance:supplier:subject:reopen",
	"idc:admin:compliance:enterprise-verification:list",
	"idc:admin:compliance:enterprise-verification:query",
	"idc:admin:compliance:enterprise-verification:review",
	"idc:admin:compliance:enterprise-verification:approve",
	"idc:admin:compliance:enterprise-verification:reject",
	"idc:admin:compliance:enterprise-material:access",
	"idc:admin:compliance:contract-template:query",
	"idc:admin:compliance:contract-template:manage",
	"idc:admin:compliance:contract:query",
	"idc:admin:compliance:contract:create",
	"idc:admin:compliance:contract:reissue",
	"idc:admin:compliance:contract:cancel-order",
	"idc:admin:compliance:contract:failure-policy",
	"idc:admin:compliance:contract:terminate",
	"idc:admin:compliance:contract-auto-sign:manage",
	"idc:admin:compliance:tax-invoice:query",
	"idc:admin:compliance:tax-invoice:review",
	"idc:admin:compliance:tax-invoice:submit",
	"idc:admin:compliance:tax-invoice:cancel",
	"idc:admin:compliance:red-letter:query",
	"idc:admin:compliance:red-letter:handle",
	"idc:admin:compliance:privacy:query",
	"idc:admin:compliance:privacy:configure",
	"idc:admin:compliance:privacy:incident",
	"idc:admin:notification:channel:manage",
	"idc:admin:product-fulfillment:task:list",
	"idc:admin:product-fulfillment:task:query",
	"idc:admin:product-fulfillment:review-approve",
	"idc:admin:product-fulfillment:review-reject",
	"idc:admin:product-fulfillment:takeover",
	"idc:admin:product-fulfillment:manual-conclude",
	"idc:admin:support:ticket:list",
	"idc:admin:support:ticket:query",
	"idc:admin:support:ticket:create",
	"idc:admin:support:ticket:reply",
	"idc:admin:support:ticket:note",
	"idc:admin:support:ticket:claim",
	"idc:admin:support:ticket:transfer",
	"idc:admin:support:ticket:resolve",
	"idc:admin:support:ticket:close",
	"idc:admin:support:ticket:reopen",
	"idc:admin:support:ticket:merge",
	"idc:admin:support:ticket:split",
	"idc:admin:support:ticket:escalate",
	"idc:admin:support:ticket:hide",
	"idc:admin:support:ticket:lead",
	"idc:admin:support:ticket:view-all",
	"idc:admin:support:ticket:merge-all",
	"idc:admin:support:ticket:upstream-forward",
	"idc:admin:support:configuration:manage",
	"idc:admin:content:manage",
	"idc:admin:service-entitlement:list",
	"idc:admin:service-entitlement:action",
	"idc:admin:service-entitlement:takeover",
	"idc:admin:service-entitlement:download",
	"idc:admin:operations:workbench:view",
	"idc:admin:operations:report:view",
	"idc:admin:operations:tasks:view",
	"idc:admin:operations:alerts:view",
	"idc:admin:sales-channel:offer:view",
	"idc:admin:sales-channel:qualification:manage",
	"idc:admin:sales-channel:assignment:manage",
	"idc:admin:sales-channel:pricing:manage",
	"idc:admin:sales-channel:commission:manage",
	"idc:admin:sales-channel:affiliate:manage",
	"idc:admin:sales-channel:commission:view",
	"idc:admin:sales-channel:commission:review",
	"idc:admin:sales-channel:commission:payout",
	"idc:admin:sales-channel:overview:view",
	"idc:admin:ai:config:view",
	"idc:admin:ai:config:manage",
	"idc:admin:ai:use",
	"idc:admin:ai:audit",
	"idc:admin:channel:token",
}

// IsAdminPermission reports whether permission is one concrete IDC management permission.
// The namespace root itself is not a permission and whitespace is not normalized implicitly.
func IsAdminPermission(permission string) bool {
	return permission == strings.TrimSpace(permission) &&
		strings.HasPrefix(permission, AdminPermissionPrefix) &&
		len(permission) > len(AdminPermissionPrefix)
}

// HasAdminPermission reports whether an effective permission snapshot contains
// at least one concrete IDC management permission. Role names and IDs are never
// accepted as substitutes for the host-published effective permission snapshot.
func HasAdminPermission(permissions []string) bool {
	for _, permission := range permissions {
		if IsAdminPermission(permission) {
			return true
		}
	}
	return false
}

// AdminPermissionCatalog returns a detached, deterministic permission list for
// bounded host authorization checks. Callers must not infer grants from roles.
func AdminPermissionCatalog() []string {
	return append([]string(nil), adminPermissionCatalog[:]...)
}

// IsCatalogedAdminPermission reports whether a concrete management permission
// is owned by the compiled IDC suite.
func IsCatalogedAdminPermission(permission string) bool {
	for _, candidate := range adminPermissionCatalog {
		if permission == candidate {
			return true
		}
	}
	return false
}
