// This file verifies the shared IDC management-permission namespace contract.

package idccap

import "testing"

// TestIsAdminPermissionRejectsNamespaceLookalikes verifies only concrete,
// exact IDC management permissions qualify as administrative authority.
func TestIsAdminPermissionRejectsNamespaceLookalikes(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{name: "concrete permission", permission: "idc:admin:customer:list", want: true},
		{name: "namespace root", permission: "idc:admin:", want: false},
		{name: "role name", permission: "admin", want: false},
		{name: "embedded prefix", permission: "other:idc:admin:customer:list", want: false},
		{name: "leading whitespace", permission: " idc:admin:customer:list", want: false},
		{name: "trailing whitespace", permission: "idc:admin:customer:list\t", want: false},
		{name: "whitespace-only suffix", permission: "idc:admin: ", want: false},
		{name: "customer permission", permission: "idc:account:profile:get", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsAdminPermission(test.permission); got != test.want {
				t.Fatalf("IsAdminPermission(%q)=%v, want %v", test.permission, got, test.want)
			}
		})
	}
}

// TestHasAdminPermissionUsesEffectiveSnapshot verifies a single qualifying
// permission in the detached host snapshot is sufficient.
func TestHasAdminPermissionUsesEffectiveSnapshot(t *testing.T) {
	if !HasAdminPermission([]string{"system:user:list", "idc:admin:server:list"}) {
		t.Fatal("expected concrete IDC administration permission to qualify")
	}
	if HasAdminPermission([]string{"admin", "system:user:list", "idc:admin:"}) {
		t.Fatal("expected role-like and namespace-root values to be rejected")
	}
}

// TestAdminPermissionCatalogIsConcreteAndDetached verifies catalog values are
// valid, unique, and cannot be mutated through a returned slice.
func TestAdminPermissionCatalogIsConcreteAndDetached(t *testing.T) {
	permissions := AdminPermissionCatalog()
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if !IsAdminPermission(permission) {
			t.Fatalf("catalog contains invalid management permission %q", permission)
		}
		if _, ok := seen[permission]; ok {
			t.Fatalf("catalog contains duplicate management permission %q", permission)
		}
		seen[permission] = struct{}{}
	}
	if len(permissions) > 0 {
		permissions[0] = "mutated"
		if AdminPermissionCatalog()[0] == "mutated" {
			t.Fatal("expected catalog result to be detached")
		}
	}
}

// TestProviderConfigurationPermissionsAreCataloged prevents the management
// runtime from rejecting the resource plugin's channel and pool administration
// endpoints after host route authorization has already succeeded.
func TestProviderConfigurationPermissionsAreCataloged(t *testing.T) {
	for _, permission := range []string{
		"idc:admin:resource:configuration:list",
		"idc:admin:resource:configuration:write",
	} {
		if !IsCatalogedAdminPermission(permission) {
			t.Fatalf("expected Provider configuration permission %q to be cataloged", permission)
		}
	}
}

// TestCurrencyPermissionsAreCataloged prevents merchant currency routes from
// being rejected by the shared management authorization catalog.
func TestCurrencyPermissionsAreCataloged(t *testing.T) {
	for _, permission := range []string{
		"idc:admin:currency:read",
		"idc:admin:currency:write",
	} {
		if !IsCatalogedAdminPermission(permission) {
			t.Fatalf("expected currency permission %q to be cataloged", permission)
		}
	}
}
