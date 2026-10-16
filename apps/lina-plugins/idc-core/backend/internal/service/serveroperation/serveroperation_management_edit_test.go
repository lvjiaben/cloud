// These tests verify that staff edits keep the existing ownership, version and exact permission boundary.

package serveroperation

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"testing"
)

// staffEditAuthorization grants only the product edit and detail permissions for this fixture.
type staffEditAuthorization struct {
	trafficTestAuthorization
	edit bool
}

func (a staffEditAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionServerAccessoryView): true, authz.PermissionKey(PermissionServerEdit): a.edit}, nil
}

func TestStaffProductEditPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	applyTrafficFixtureMigration(t, fixture, "065-server-customer-metadata.sql")
	authorization := staffEditAuthorization{edit: true}
	fixture.service.authorization = authorization
	operations := &serviceImpl{database: fixture.database, audit: fixture.service.audit}
	management := &managementServiceImpl{bizCtx: fixture.service.bizCtx, principal: fixture.service.principal, authorization: authorization, directory: &directoryImpl{database: fixture.database}, operations: operations}
	before, err := management.Metadata(fixture.ctx, "SRV_checkout_original")
	if err != nil || before == nil {
		t.Fatalf("metadata: %+v %v", before, err)
	}
	updated, err := management.UpdateMetadata(fixture.ctx, "SRV_checkout_original", "报表服务器", "客户备注", before.Version)
	if err != nil || updated.Version != before.Version+1 || updated.Remark != "客户备注" {
		t.Fatalf("edit: %+v %v", updated, err)
	}
	if _, err = management.UpdateMetadata(fixture.ctx, "SRV_checkout_original", "覆盖", "过期修改", before.Version); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	after, err := management.Metadata(fixture.ctx, "SRV_checkout_original")
	if err != nil || after.Remark != "客户备注" {
		t.Fatalf("stale edit changed note: %+v %v", after, err)
	}
	management.authorization = staffEditAuthorization{edit: false}
	if _, err = management.UpdateMetadata(fixture.ctx, "SRV_checkout_original", "越权", "", after.Version); !bizerr.Is(err, CodeAccessoryManagementNotAccessible) {
		t.Fatalf("missing edit grant: %v", err)
	}
	fixture.service.authorization = staffEditAuthorization{edit: false}
	if _, err = fixture.service.ResizeOptionsForStaff(fixture.ctx, "SRV_checkout_original"); !bizerr.Is(err, CodeAccessoryManagementNotAccessible) {
		t.Fatalf("read resize without edit grant: %v", err)
	}
	if _, err = fixture.service.PreviewResizeForStaff(fixture.ctx, ResizeQuoteInput{ServiceNo: "SRV_checkout_original"}); !bizerr.Is(err, CodeAccessoryManagementNotAccessible) {
		t.Fatalf("quote without edit grant: %v", err)
	}
	if _, err = fixture.service.CheckoutResizeForStaff(fixture.ctx, "SRV_checkout_original", ResizeCheckoutInput{QuoteNo: "other-quote"}); !bizerr.Is(err, CodeAccessoryManagementNotAccessible) {
		t.Fatalf("checkout without edit grant: %v", err)
	}
	fixture.service.authorization = authorization
	if _, err = fixture.service.CheckoutResizeForStaff(fixture.ctx, "SRV_checkout_original", ResizeCheckoutInput{QuoteNo: "other-quote"}); err == nil {
		t.Fatal("unowned quote accepted")
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_billing_invoice`, 0)
}
