package v1

import (
	"reflect"
	"testing"
)

func TestCatalogFulfillmentPolicyRequestDoesNotExposeExecutionInternals(t *testing.T) {
	t.Parallel()
	typeOf := reflect.TypeOf(CatalogFulfillmentPolicyUpdateReq{})
	allowed := map[string]struct{}{"Meta": {}, "VersionNo": {}, "StrategyCode": {}}
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		if _, ok := allowed[field.Name]; !ok {
			t.Fatalf("merchant request exposes unexpected field %s", field.Name)
		}
	}
	for _, forbidden := range []string{
		"GranularityCode", "ProvisionMethod", "QueryMethod", "MaximumAttempts", "RetryBaseSeconds",
	} {
		if _, exists := typeOf.FieldByName(forbidden); exists {
			t.Fatalf("merchant request exposes technical field %s", forbidden)
		}
	}
}

func TestCatalogFulfillmentPolicySummaryDoesNotReturnMethodsOrRetrySettings(t *testing.T) {
	t.Parallel()
	typeOf := reflect.TypeOf(CatalogFulfillmentPolicySummary{})
	for _, forbidden := range []string{
		"GranularityCode", "ProvisionMethod", "QueryMethod", "MaximumAttempts", "RetryBaseSeconds", "PolicyDigest",
	} {
		if _, exists := typeOf.FieldByName(forbidden); exists {
			t.Fatalf("merchant summary exposes technical field %s", forbidden)
		}
	}
}
