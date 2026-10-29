// This file reads bounded local automation facts after fulfillment detail
// authorization. It never reads another plugin's tables or calls a supplier.
package productfulfillment

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"regexp"
	"strings"

	"lina-plugin-idc-core/backend/internal/model/do"
)

const resourceEntitlementOwnerID = "idc-resource"

// diagnosticCodePattern excludes free text and payloads from diagnostic codes.
var diagnosticCodePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{0,96}$`)
var diagnosticResourceTaskPattern = regexp.MustCompile(`^task_[a-f0-9]{32}$`)

// loadExecutionDiagnostic is called only after the intent passed detail scope
// authorization. The tenant/intent key and task/attempt index bound this to two local reads.
func (s *serviceImpl) loadExecutionDiagnostic(ctx context.Context, intentNo string) (*ExecutionDiagnostic, error) {
	var row *struct {
		ExecutionDiagnostic
		TaskNo string
	}
	err := s.database.Model("plugin_idc_core_product_fulfillment_task").Ctx(ctx).
		Where(do.ProductFulfillmentTask{TenantId: 0, IntentNo: intentNo}).
		Fields("task_no,external_reference AS resource_task_no,status_code,phase_code,last_result_code,attempt_count").
		Limit(1).Scan(&row)
	if err != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	if row == nil {
		return nil, nil
	}
	// Only the latest closed attempt for this authorized local task is read.
	// Supplier free text and all other evidence remain outside this projection.
	var facts *struct {
		FailureCode     string
		ResponseExcerpt string
	}
	err = s.database.GetScan(ctx, &facts, `SELECT safe_result_evidence->>'failureCode' AS failure_code, safe_result_evidence->>'responseExcerpt' AS response_excerpt
FROM plugin_idc_core_product_fulfillment_attempt
WHERE tenant_id=0 AND task_no=? AND finished_at IS NOT NULL
ORDER BY attempt_no DESC LIMIT 1`, row.TaskNo)
	if err != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	if facts != nil {
		row.FailureCode = safeDiagnosticFailureCode(facts.FailureCode)
		row.ResponseExcerpt = facts.ResponseExcerpt
	}
	return sanitizeExecutionDiagnostic(&row.ExecutionDiagnostic), nil
}

// sanitizeExecutionDiagnostic projects references and machine codes only; an
// invalid reference is omitted rather than exposed or used to query a task.
func sanitizeExecutionDiagnostic(input *ExecutionDiagnostic) *ExecutionDiagnostic {
	if input == nil {
		return nil
	}
	result := *input
	result.FailureCode = safeDiagnosticFailureCode(result.FailureCode)
	// The resource owner already credential-scanned this dedicated field. Core
	// only accepts bounded JSON, and the frontend renders it as escaped text.
	if len(result.ResponseExcerpt) > 4096 || !json.Valid([]byte(result.ResponseExcerpt)) {
		result.ResponseExcerpt = ""
	}
	if !diagnosticResourceTaskPattern.MatchString(result.ResourceTaskNo) {
		result.ResourceTaskNo = ""
	}
	for _, code := range []*string{&result.StatusCode, &result.PhaseCode, &result.LastResultCode} {
		if !diagnosticCodePattern.MatchString(*code) {
			*code = ""
		}
	}
	return &result
}

// safeDiagnosticFailureCode accepts reviewed categories only, never arbitrary
// provider codes which could carry private values even without punctuation.
func safeDiagnosticFailureCode(code string) string {
	code = strings.ToLower(code)
	switch code {
	case string(provisionresourcecap.PreparationFailureRequest), string(provisionresourcecap.PreparationFailureLookup),
		string(provisionresourcecap.PreparationFailureParameters), string(provisionresourcecap.PreparationFailureCredential),
		string(provisionresourcecap.PreparationFailureCallback), string(provisionresourcecap.PreparationFailureTaskInput),
		string(provisionresourcecap.PreparationFailureTaskStorage), provisionresourcecap.FailureRequestNotSent, "upstream_request_rejected", "upstream_result_unknown", "upstream_task_failed",
		"idc_resource_provider_timeout", "idc_resource_provider_dns", "idc_resource_provider_network",
		"idc_resource_provider_endpoint_not_found", "idc_resource_provider_invalid_response",
		"idc_resource_provider_auth_rejected", "idc_resource_provider_local_auth_policy":
		return code
	default:
		return ""
	}
}
