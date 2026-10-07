package electroniccontractpreparation

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

// The read test uses the existing preparation migrations and restores the DAO group.
func TestManagementContractNumberedPagesPostgreSQL(t *testing.T) {
	database, cleanup := newPreparationTestDatabase(t)
	defer cleanup()
	original := gdb.GetAllConfig()
	if err := gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{*database.GetConfig()}}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gdb.SetConfig(original) }()
	ctx := context.Background()
	for index := 0; index < 26; index++ {
		tenant := 0
		if index == 25 {
			tenant = 9
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_contract
   (tenant_id, contract_no, customer_no, principal_no, order_no, contract_type_code, template_no, template_version,
    variable_snapshot_ref, variable_snapshot_sha256, signer_snapshot_sha256, currency_code, amount_minor, status_code, snapshot_key_ref)
   VALUES (?, ?, 'CUSTOMER_001', 'PRINCIPAL_001', ?, 'service_order', 'TPL_001', 2, 'private://test', repeat('a',64), repeat('b',64), 'CNY', 1000, 'draft', 'secret:test')`,
			tenant, fmt.Sprintf("CTR_PAGE_%02d", index), fmt.Sprintf("ORDER_PAGE_%02d", index))
		if err != nil {
			t.Fatal(err)
		}
	}
	authorizer := &preparationAuthorizer{allowed: true}
	service := &serviceImpl{database: database, authorizer: authorizer}
	for _, size := range []int{10, 20, 50, 100} {
		seen := map[string]bool{}
		for pageNumber := 1; pageNumber <= (25+size-1)/size+1; pageNumber++ {
			page, err := service.ListManagementContracts(ctx, ManagementContractQuery{Page: pageNumber, PageSize: size})
			if err != nil {
				t.Fatalf("size=%d page=%d: %+v", size, pageNumber, err)
			}
			expected := min(size, max(25-(pageNumber-1)*size, 0))
			if page.Total != 25 || len(page.Items) != expected {
				t.Fatalf("size=%d page=%d total=%d rows=%d", size, pageNumber, page.Total, len(page.Items))
			}
			for _, item := range page.Items {
				if seen[item.ContractNo] {
					t.Fatalf("duplicate row %s", item.ContractNo)
				}
				seen[item.ContractNo] = true
			}
		}
		if len(seen) != 25 {
			t.Fatalf("missing rows: %d", len(seen))
		}
	}
	filtered, err := service.ListManagementContracts(ctx, ManagementContractQuery{Page: 1, PageSize: 10, OrderNo: "ORDER_PAGE_01"})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("filtered page=%+v err=%+v", filtered, err)
	}
	authorizer.allowed = false
	if _, err = service.ListManagementContracts(ctx, ManagementContractQuery{Page: 1, PageSize: 10}); err == nil {
		t.Fatal("denied list returned data")
	}
}
