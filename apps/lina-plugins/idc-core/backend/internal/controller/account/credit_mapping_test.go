// This file verifies canonical customer credit amounts and response-field isolation.

package account

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func TestParsePositiveCreditAmountRequiresCanonicalPositiveInteger(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "+1", "01", "1.0", "9223372036854775808"} {
		if _, err := parsePositiveCreditAmount(value); !bizerr.Is(err, billing.CodeCreditInvalid) {
			t.Fatalf("value %q must fail with the credit validation code: %v", value, err)
		}
	}
	if value, err := parsePositiveCreditAmount("9223372036854775807"); err != nil || value != int64(9223372036854775807) {
		t.Fatalf("maximum int64 minor amount was not preserved: value=%d err=%v", value, err)
	}
}

func TestCreditApplicationIntentValidatesUploadCollectionCardinality(t *testing.T) {
	t.Parallel()
	request := func(uploadCount int) *v1.CreditApplicationSubmitReq {
		uploadNos := make([]string, uploadCount)
		for index := range uploadNos {
			uploadNos[index] = "CEU_test_" + strconv.Itoa(index)
		}
		return &v1.CreditApplicationSubmitReq{
			CurrencyCode: "CNY", RequestedLimitMinor: "10000",
			RequestedValidFrom: 1_786_406_400_000, RequestedValidUntil: 1_817_942_400_000,
			UploadNos: uploadNos, IdempotencyKey: "credit-upload-boundary-test",
		}
	}
	for _, count := range []int{minimumCreditApplicationUploads, maximumCreditApplicationUploads} {
		if _, err := creditApplicationIntent(request(count)); err != nil {
			t.Fatalf("upload count %d must pass HTTP mapping: %v", count, err)
		}
	}
	for _, count := range []int{0, maximumCreditApplicationUploads + 1} {
		if _, err := creditApplicationIntent(request(count)); !bizerr.Is(err, billing.CodeCreditInvalid) {
			t.Fatalf("upload count %d must fail with stable credit validation: %v", count, err)
		}
	}
}

func TestAccountCreditMappingDoesNotExposeCustomerNumber(t *testing.T) {
	now := time.UnixMilli(1_786_406_400_123)
	mapped := toAPICreditApplicationDirectory(billing.CreditApplicationDirectoryItem{
		ApplicationNo: "CAP_mapping_1", CustomerNo: "CUS_private", CurrencyCode: "CNY",
		PolicyNo: "CRP_mapping_1", PolicyVersion: 1, RequestedLimitMinor: 100_000,
		RequestedValidFrom: now, RequestedValidUntil: now.Add(24 * time.Hour),
		RequiredApprovalCount: 1, StatusCode: "pending_review", CreatedAt: now, UpdatedAt: now,
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("marshal customer credit application: %v", err)
	}
	if strings.Contains(string(encoded), "customerNo") || strings.Contains(string(encoded), "CUS_private") {
		t.Fatalf("customer projection exposed staff-only ownership data: %s", encoded)
	}
}
