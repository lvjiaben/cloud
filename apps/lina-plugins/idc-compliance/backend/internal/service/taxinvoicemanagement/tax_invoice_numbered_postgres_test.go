package taxinvoicemanagement

import (
	"context"
	"fmt"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"testing"
)

func TestTaxInvoiceNumberedPagesPostgreSQL(t *testing.T) {
	database := openManagementTestDatabase(t)
	service, err := New(database, managementcap.Runtime{Authorizer: new(managementAuthorizer), Audit: new(managementAudit)}, ClosedNuoNuoGate{}, managementSnapshotAccessor{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for index := 0; index < 25; index++ {
		no := fmt.Sprintf("TINV_PAGE_%02d", index)
		insertManagementRequest(t, database, no, StatusPendingReview, 1, "", false, false)
	}
	for _, size := range []int{10, 20, 50, 100} {
		seen := map[string]bool{}
		for number := 1; number <= (25+size-1)/size+1; number++ {
			page, err := service.List(ctx, ListQuery{Page: number, Limit: size})
			if err != nil {
				t.Fatalf("size=%d page=%d: %+v", size, number, err)
			}
			if page.Total != 25 || len(page.Items) != min(size, max(25-(number-1)*size, 0)) {
				t.Fatalf("size=%d page=%d total=%d rows=%d", size, number, page.Total, len(page.Items))
			}
			for _, item := range page.Items {
				if seen[item.TaxInvoiceNo] {
					t.Fatalf("duplicate %s", item.TaxInvoiceNo)
				}
				seen[item.TaxInvoiceNo] = true
			}
		}
		if len(seen) != 25 {
			t.Fatalf("missing rows=%d", len(seen))
		}
	}
	filtered, err := service.List(ctx, ListQuery{Page: 1, Limit: 10, CustomerNo: "TINV_PAGE_01customer"})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("filtered=%+v err=%+v", filtered, err)
	}
	empty, err := service.List(ctx, ListQuery{Page: 1, Limit: 10, StatusCode: StatusCanceled})
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty=%+v err=%+v", empty, err)
	}
}
