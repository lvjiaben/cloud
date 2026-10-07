// This file verifies customer projection inputs for management ledger transactions.

package billing

import "testing"

func TestManagementTransactionCustomerNosExcludesMerchantOwnedEntries(t *testing.T) {
	items := []ManagementTransactionDirectoryItem{
		{TransactionNo: "TXN_MERCHANT", CustomerNo: ""},
		{TransactionNo: "TXN_CUSTOMER", CustomerNo: "CUS_TEST"},
	}

	got := managementTransactionCustomerNos(items)
	if len(got) != 1 || got[0] != "CUS_TEST" {
		t.Fatalf("unexpected customer projection input: %#v", got)
	}
}
