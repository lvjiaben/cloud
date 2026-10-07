package contractlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

// syncFailureCase freezes the first terminal event and active handling policy
// in the same transaction as the legal timeline. A later authoritative
// completion resolves only work that has not started compensation.
func (s *serviceImpl) syncFailureCase(
	ctx context.Context,
	locked lockedFlow,
	observation Observation,
	decision string,
	contractStatus string,
	now time.Time,
) error {
	if decision != "applied" {
		return nil
	}
	if contractStatus == "completed" {
		_, err := s.contracts.Exec(ctx, `
WITH RECURSIVE completed_lineage(contract_no) AS (
    SELECT ?::VARCHAR(64)
    UNION
    SELECT replacement.old_contract_no
    FROM plugin_idc_compliance_contract_replacement replacement
    JOIN completed_lineage child
      ON child.contract_no = replacement.new_contract_no
    WHERE replacement.tenant_id = 0
)
UPDATE plugin_idc_compliance_contract_failure_case
SET status_code = 'signed', completed_at = ?, updated_at = ?,
    last_reason_code = 'authoritative_contract_completed'
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'pending'
  AND contract_no IN (SELECT contract_no FROM completed_lineage)`,
			locked.ContractNo, now, now, locked.OrderNo)
		return gerror.Wrap(err, "resolve pending contract failure case")
	}
	if !terminalFailureStatus(contractStatus) {
		return nil
	}
	eventDigest := sha256.Sum256([]byte(
		locked.ContractNo + "\x00" + observation.RouteNo + "\x00" +
			strconv.FormatInt(observation.RouteVersion, 10),
	))
	eventHex := hex.EncodeToString(eventDigest[:])
	terminalEventNo := "cfe_" + eventHex[:32]
	caseNo := "cfc_" + eventHex[:32]
	result, err := s.contracts.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_failure_case
    (tenant_id, case_no, contract_no, order_no, terminal_event_no,
     terminal_status_code, policy_no, policy_version, handling_days,
     handling_deadline, status_code, created_at, updated_at)
SELECT 0, ?, contract.contract_no, contract.order_no, ?, ?,
       policy.policy_no, policy.policy_version, policy.handling_days,
       ?::timestamptz + policy.handling_days * INTERVAL '1 day',
       'pending', ?, ?
FROM plugin_idc_compliance_contract contract
CROSS JOIN LATERAL (
    SELECT policy_no, policy_version, handling_days
    FROM plugin_idc_compliance_contract_failure_policy
    WHERE tenant_id = 0 AND status_code = 'active'
    ORDER BY policy_version DESC
    LIMIT 1
) policy
WHERE contract.tenant_id = 0 AND contract.contract_no = ?
  AND contract.required_for_fulfillment
ON CONFLICT (contract_no) DO NOTHING`,
		caseNo, terminalEventNo, contractStatus, now, now, now, locked.ContractNo)
	if err != nil {
		return gerror.Wrap(err, "freeze required contract failure case")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count frozen contract failure case")
	}
	if affected == 0 {
		var count int
		if scanErr := s.contracts.Ctx(ctx).Raw(`
SELECT COUNT(*) FROM plugin_idc_compliance_contract_failure_case
WHERE tenant_id = ? AND contract_no = ? AND terminal_event_no = ?
  AND terminal_status_code = ?`, idccap.DefaultTenantID, locked.ContractNo,
			terminalEventNo, contractStatus).Scan(&count); scanErr != nil || count != 1 {
			return gerror.New("required contract failure case conflicts with existing terminal event")
		}
	}
	return nil
}
