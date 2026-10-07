package notificationpolicy

import (
	"testing"

	"lina-core/pkg/plugin/capability/bizctxcap"
)

func TestCustomerNotificationAudienceDoesNotRequireStaffDataScope(t *testing.T) {
	current := bizctxcap.CurrentContext{
		UserID:               12,
		TenantID:             0,
		DataScopeUnsupported: true,
	}
	if !validCurrentCustomerContext(current) {
		t.Fatal("customer self-service must not require a staff data scope")
	}
	current.IsImpersonation = true
	if validCurrentCustomerContext(current) {
		t.Fatal("incomplete impersonation context must not enter customer notifications")
	}
	current.ActingUserID = 7
	if !validCurrentCustomerContext(current) {
		t.Fatal("represented customer must retain ordinary customer notification access")
	}
	current.ActingAsTenant = true
	if validCurrentCustomerContext(current) {
		t.Fatal("tenant impersonation must not enter customer notifications")
	}
}
