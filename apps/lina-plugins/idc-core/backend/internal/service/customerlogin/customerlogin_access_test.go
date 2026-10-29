// This file verifies that access repair never alters custom roles, staff,
// disabled users or customer accounts outside the original roleless path.
package customerlogin

import (
	"reflect"
	"testing"
)

func TestAccessRepairPreviewExcludesCustomAndFrozenAccounts(t *testing.T) {
	s := &serviceImpl{}
	profiles := []hostProfile{{ID: 1, TenantID: 0, Status: 1}, {ID: 2, TenantID: 0, Status: 1, RoleIDs: []int{99}}, {ID: 3, TenantID: 0, Status: 0}, {ID: 4, TenantID: 1, Status: 1}, {ID: 5, TenantID: 0, Status: 1}, {ID: 6, TenantID: 0, Status: 1}, {ID: 7, TenantID: 0, Status: 1}}
	owners := map[int]ownedIdentity{1: {SourceCode: adminDirectSource, CustomerStatus: activeCustomerStatus}, 2: {SourceCode: adminDirectSource, CustomerStatus: activeCustomerStatus}, 3: {SourceCode: adminDirectSource, CustomerStatus: activeCustomerStatus}, 4: {SourceCode: adminDirectSource, CustomerStatus: activeCustomerStatus}, 5: {SourceCode: "invitation", CustomerStatus: activeCustomerStatus}, 6: {SourceCode: adminDirectSource, CustomerStatus: "restricted"}}
	candidates, err := s.repairAccess(t.Context(), profiles, owners, false)
	if err != nil || !reflect.DeepEqual(candidates, []int{1}) {
		t.Fatalf("unexpected repair targets %v: %v", candidates, err)
	}
	if !reflect.DeepEqual(profiles[1].RoleIDs, []int{99}) {
		t.Fatal("preview changed custom roles")
	}
}
