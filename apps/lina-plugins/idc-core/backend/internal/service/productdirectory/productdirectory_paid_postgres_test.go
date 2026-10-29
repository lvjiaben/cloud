// This file verifies paid-only purchased rows, totals and independent plugin blocks.
package productdirectory

import "testing"

func TestPaidServicesAndProviderBlocksPostgreSQL(t *testing.T) {
	db, service, _, _ := newDirectoryFixture(t)
	ctx := t.Context()
	menu, err := service.Save(ctx, SaveInput{NameZhCn: "服务", NameEnUs: "Services", NavigationType: NavigationProduct})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_directory_membership(tenant_id,product_no,directory_no) VALUES(0,'product-a',?),(0,'product-c',?)`, menu.DirectoryNo, menu.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET entitlement_owner_id='idc-resource' WHERE entitlement_no='ent-a';UPDATE plugin_idc_core_service_entitlement SET entitlement_owner_id='idc-core',plugin_id='yunsu-ssl' WHERE entitlement_no='ent-c';INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,product_no,entitlement_owner_id,plugin_id,status_code,order_no) VALUES('unpaid-a','customer-a','product-a','idc-resource','cloud','active','unpaid-order'),('paid-pending','customer-a','product-a','idc-resource','cloud','pending_activation','pending-order'),('foreign-paid','customer-a','product-a','idc-resource','cloud','active','foreign-order');INSERT INTO plugin_idc_core_commerce_order(tenant_id,order_no,customer_no,paid_at) VALUES(0,'unpaid-order','customer-a',NULL),(0,'pending-order','customer-a',CURRENT_TIMESTAMP),(0,'foreign-order','customer-b',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListServices(ctx, menu.DirectoryNo, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 1 || len(page.Providers) != 2 {
		t.Fatalf("paid page=%+v", page)
	}
	counts := map[string]int{}
	for _, p := range page.Providers {
		counts[p.PluginID] = p.Total
	}
	if counts["idc-resource"] != 2 || counts["yunsu-ssl"] != 1 {
		t.Fatal(counts)
	}
	native, err := service.ListProviderServices(ctx, menu.DirectoryNo, 1, 100, "", "idc-resource")
	if err != nil || native.Total != 2 {
		t.Fatalf("native=%+v err=%v", native, err)
	}
	for _, item := range native.Items {
		if item.EntitlementNo == "unpaid-a" || item.EntitlementNo == "foreign-paid" || item.EntitlementNo == "ent-c" {
			t.Fatalf("foreign or unpaid row=%+v", item)
		}
	}
	ssl, err := service.ListProviderServices(ctx, menu.DirectoryNo, 1, 100, "", "yunsu-ssl")
	if err != nil || ssl.Total != 1 || ssl.Items[0].EntitlementNo != "ent-c" {
		t.Fatalf("ssl=%+v err=%v", ssl, err)
	}
	empty, err := service.ListServices(ctx, menu.DirectoryNo, 1, 100, "unpaid-a")
	if err != nil || empty.Total != 0 || len(empty.Providers) != 0 {
		t.Fatalf("unpaid search=%+v %v", empty, err)
	}
	menus, err := service.ListAccount(ctx, 1, 100, NavigationProduct)
	if err != nil || len(menus.Items) != 1 || menus.Items[0].ServiceCount != 3 {
		t.Fatalf("count=%+v err=%v", menus, err)
	}
}
