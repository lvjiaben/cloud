package taxinvoiceredletter

import (
	"context"
	"fmt"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
	"testing"
)

func TestRedLetterNumberedPagesPostgreSQL(t *testing.T) {
	database := openRedLetterTestDatabase(t)
	ctx := context.Background()
	source := &redLetterSource{operations: map[string]*taxinvoicecap.RefundOperation{}}
	audit := new(redLetterAudit)
	service, err := New(database, source, &redLetterCustomer{customerNo: "customer-1"}, managementcap.Runtime{Authorizer: new(redLetterAuthorizer), Audit: audit}, audit, audit, new(redLetterGateway))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 25; index++ {
		key := fmt.Sprintf("page-%02d", index)
		seedIssuedTaxInvoice(t, database, "tax-"+key, "billing-"+key, "item-"+key, 1000, 900)
		source.operations["refund-"+key] = postedRefund("refund-"+key, "customer-1", "allocation-"+key, "billing-"+key, "item-"+key, 400)
		if _, err = service.RecordPostedRefund(ctx, "refund-"+key, "record-numbered-refund-"+key); err != nil {
			t.Fatal(err)
		}
	}
	for _, size := range []int{10, 20, 50, 100} {
		seen := map[string]bool{}
		for number := 1; number <= (25+size-1)/size+1; number++ {
			page, err := service.ListAdmin(ctx, AdminListQuery{Page: number, Limit: size})
			if err != nil {
				t.Fatalf("size=%d page=%d: %+v", size, number, err)
			}
			if page.Total != 25 || len(page.Items) != min(size, max(25-(number-1)*size, 0)) {
				t.Fatalf("size=%d page=%d total=%d rows=%d", size, number, page.Total, len(page.Items))
			}
			for _, item := range page.Items {
				if seen[item.TaskNo] {
					t.Fatalf("duplicate %s", item.TaskNo)
				}
				seen[item.TaskNo] = true
			}
		}
		if len(seen) != 25 {
			t.Fatalf("missing rows=%d", len(seen))
		}
	}
	empty, err := service.ListAdmin(ctx, AdminListQuery{Page: 1, Limit: 10, CustomerNo: "other-customer"})
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty=%+v err=%+v", empty, err)
	}
}
