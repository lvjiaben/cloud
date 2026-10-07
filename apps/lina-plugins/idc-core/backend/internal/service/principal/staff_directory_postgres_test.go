package principal

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func TestStaffDirectoryReaderResolvesBoundedRowsAgainstPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	insert := func(code AudienceCode, status AudienceStatus) int {
		t.Helper()
		userID := newIntegrationUserID(t)
		if _, err := dao.PrincipalAudience.Ctx(ctx).Data(do.PrincipalAudience{
			TenantId: idccap.DefaultTenantID, SysUserId: userID,
			AudienceCode: string(code), StatusCode: string(status),
			CreatedBy: userID, UpdatedBy: userID,
		}).Insert(); err != nil {
			t.Fatalf("insert staff directory fixture: %v", err)
		}
		t.Cleanup(func() { cleanupPrincipalIntegrationRows(t, ctx, userID, "") })
		return userID
	}
	activeID := insert(AudienceStaff, AudienceStatusActive)
	revokedID := insert(AudienceStaff, AudienceStatusRevoked)
	customerID := insert(AudienceCustomer, AudienceStatusActive)
	missingID := newIntegrationUserID(t)

	states, err := NewStaffDirectoryReader().ResolveBatch(ctx, []int{activeID, revokedID, customerID, missingID})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 3 {
		t.Fatalf("resolved states=%d want=3", len(states))
	}
	if active := states[activeID]; active.Code != AudienceStaff || active.Status != AudienceStatusActive || active.Version <= 0 || active.HasCustomerBinding {
		t.Fatalf("unexpected active state: %+v", active)
	}
	if revoked := states[revokedID]; revoked.Code != AudienceStaff || revoked.Status != AudienceStatusRevoked || revoked.Version <= 0 {
		t.Fatalf("unexpected revoked state: %+v", revoked)
	}
	if customer := states[customerID]; customer.Code != AudienceCustomer || customer.Version <= 0 {
		t.Fatalf("unexpected customer state: %+v", customer)
	}
}
