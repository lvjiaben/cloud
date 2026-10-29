// This file verifies implicit direct routing without overriding merchant pools.
package provisionsource

import (
	"lina-plugin-idc-resource/backend/internal/model/do"
	"testing"
)

func TestDefaultDirectRouteIsIdempotentAndPreservesMerchantPools(t *testing.T) {
	ctx, db := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, db)
	source, err := New(db, newProvisionSecretSource(t), principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	// The fixture already has an explicit pool. It must remain the only route.
	if err = source.ensureDefaultInterfacePool(ctx, request.InterfaceInstanceNo, request.InterfaceConfigurationRevision); err != nil {
		t.Fatal(err)
	}
	before, err := db.Model(do.Pool{}).Ctx(ctx).Count()
	if err != nil || before != 1 {
		t.Fatal("merchant pool changed", err)
	}
	// A second fresh channel uses its own revision and has no configured pools.
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,display_name,endpoint,status_code) SELECT 'CHN_direct00000000001',provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,'Direct','https://supplier.example','enabled' FROM plugin_idc_resource_channel LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding(interface_instance_no,channel_no,binding_version,current_configuration_revision,product_plugin_id,product_plugin_release_version,product_contract_version,product_authorized_methods,product_release_digest,product_declaration_digest) SELECT 'pfi_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee','CHN_direct00000000001',1,'rci_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',product_plugin_id,product_plugin_release_version,product_contract_version,product_authorized_methods,product_release_digest,product_declaration_digest FROM plugin_idc_resource_interface_channel_binding LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision(revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by) VALUES('rci_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee','pfi_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee','CHN_direct00000000001','https://supplier.example','{}','{}',101)`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = source.ensureDefaultInterfacePool(ctx, "pfi_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "rci_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); err != nil {
			t.Fatal(err)
		}
	}
	count, err := db.Model(do.Pool{}).Ctx(ctx).Where(do.Pool{ChannelNo: "CHN_direct00000000001"}).Count()
	if err != nil || count != 1 {
		t.Fatal("duplicate direct pool", err)
	}
	var row struct{ Selector, StatusCode string }
	if err = db.Model(do.Pool{}).Ctx(ctx).Fields("selector::text AS selector,status_code").Where(do.Pool{ChannelNo: "CHN_direct00000000001"}).Scan(&row); err != nil || row.StatusCode != "enabled" {
		t.Fatal("direct pool unavailable", err)
	}
	// An explicit disabled pool stays disabled; no extra active route is inserted.
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_resource_pool SET status_code='disabled',version=version+1 WHERE channel_no='CHN_direct00000000001'`); err != nil {
		t.Fatal(err)
	}
	if err = source.ensureDefaultInterfacePool(ctx, "pfi_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "rci_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); err != nil {
		t.Fatal(err)
	}
	active, err := db.Model(do.Pool{}).Ctx(ctx).Where(do.Pool{ChannelNo: "CHN_direct00000000001", StatusCode: "enabled"}).Count()
	if err != nil || active != 0 {
		t.Fatal("disabled merchant route bypassed", err)
	}
}
