// This file appends customer-owned automatic renewal policies. Only recurring
// paid services are eligible; trial services cannot authorize automatic charges.

package renewal

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type policyEntitlementRow struct {
	EntitlementNo         string  `orm:"entitlement_no"`
	CustomerNo            string  `orm:"customer_no"`
	EntitlementStatusCode string  `orm:"entitlement_status_code"`
	ChargeTypeCode        string  `orm:"charge_type_code"`
	PeriodUnitCode        string  `orm:"period_unit_code"`
	RenewalModeCode       string  `orm:"renewal_mode_code"`
	EntitlementVersion    int64   `orm:"entitlement_version"`
	PolicyNo              *string `orm:"policy_no"`
	PolicyVersion         *int64  `orm:"policy_version"`
	AutoRenewEnabled      *bool   `orm:"auto_renew_enabled"`
	RenewalCycles         *int    `orm:"renewal_cycles"`
	LeadSeconds           *int64  `orm:"lead_seconds"`
	MaximumAttempts       *int    `orm:"maximum_attempts"`
	RetryIntervalSeconds  *int64  `orm:"retry_interval_seconds"`
	FailureNotification   *bool   `orm:"failure_notification_enabled"`
	ConfiguredBy          *int    `orm:"configured_by"`
}

// ConfigurePolicyCurrentCustomer appends a policy revision and atomically
// advances only the owned entitlement pointer. Exact current-state replay does not
// create duplicate history.
func (s *serviceImpl) ConfigurePolicyCurrentCustomer(
	ctx context.Context,
	input PolicyInput,
) (*Policy, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive ||
		!validReference(audience.CustomerNo, 40) || audience.UserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if input.EntitlementNo != strings.TrimSpace(input.EntitlementNo) ||
		!validReference(input.EntitlementNo, 40) || input.ExpectedEntitlementVersion < 1 ||
		input.RenewalCycles < 1 || input.RenewalCycles > 120 ||
		input.LeadSeconds < 3600 || input.LeadSeconds > 31_536_000 ||
		input.MaximumAttempts < 1 || input.MaximumAttempts > 32 ||
		input.RetryIntervalSeconds < 300 || input.RetryIntervalSeconds > 2_592_000 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Policy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := lockPolicyEntitlement(
			ctx, s.database, audience.CustomerNo, input.EntitlementNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if row.ChargeTypeCode != string(catalog.ChargeTypeRecurring) || row.PeriodUnitCode == "once" ||
			(row.EntitlementStatusCode != "active" && row.EntitlementStatusCode != "expired") ||
			(row.RenewalModeCode != "local_entitlement" &&
				row.RenewalModeCode != "provider_confirmed") {
			return bizerr.NewCode(CodeCapabilityUnavailable)
		}
		if samePolicyInput(row, audience.UserID, input) {
			result = projectPolicy(row, true)
			return nil
		}
		if row.EntitlementVersion != input.ExpectedEntitlementVersion {
			return bizerr.NewCode(CodeConflict)
		}
		policyVersion := int64(1)
		if row.PolicyVersion != nil {
			policyVersion = *row.PolicyVersion + 1
		}
		policyNo, numberErr := newNumber(s.random, "RPL_")
		if numberErr != nil {
			return numberErr
		}
		content, digestErr := contentDigest(struct {
			PolicyNo, EntitlementNo string
			PolicyVersion           int64
			Input                   PolicyInput
			ConfiguredBy            int
		}{policyNo, row.EntitlementNo, policyVersion, input, audience.UserID})
		if digestErr != nil {
			return digestErr
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_renewal_policy
    (tenant_id, policy_no, entitlement_no, policy_version, auto_renew_enabled,
     renewal_cycles, lead_seconds, maximum_attempts, retry_interval_seconds,
     failure_notification_enabled, configured_by, content_digest)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, policyNo, row.EntitlementNo,
			policyVersion, input.AutoRenewEnabled, input.RenewalCycles,
			input.LeadSeconds, input.MaximumAttempts, input.RetryIntervalSeconds,
			input.FailureNotificationEnabled, audience.UserID, content); err != nil {
			return gerror.Wrap(err, "insert entitlement renewal policy")
		}
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_service_entitlement
SET current_renewal_policy_no = ?, current_renewal_policy_version = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND entitlement_no = ? AND customer_no = ? AND version = ?`,
			policyNo, policyVersion, row.EntitlementNo, audience.CustomerNo, row.EntitlementVersion)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "advance entitlement renewal policy")
		}
		affected, affectedErr := updated.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result = &Policy{
			PolicyNo: policyNo, EntitlementNo: row.EntitlementNo, PolicyVersion: policyVersion,
			EntitlementVersion: row.EntitlementVersion + 1,
			AutoRenewEnabled:   input.AutoRenewEnabled, RenewalCycles: input.RenewalCycles,
			LeadSeconds: input.LeadSeconds, MaximumAttempts: input.MaximumAttempts,
			RetryIntervalSeconds:       input.RetryIntervalSeconds,
			FailureNotificationEnabled: input.FailureNotificationEnabled,
		}
		return nil
	})
	return result, normalizeError(err)
}

func lockPolicyEntitlement(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	entitlementNo string,
) (*policyEntitlementRow, error) {
	var row *policyEntitlementRow
	if err := database.GetScan(ctx, &row, `
SELECT service.entitlement_no AS entitlement_no, service.customer_no, service.status_code AS entitlement_status_code,
       service.charge_type_code, service.period_unit_code, service.renewal_mode_code,
       service.version AS entitlement_version,
       policy.policy_no, policy.policy_version, policy.auto_renew_enabled,
       policy.renewal_cycles, policy.lead_seconds, policy.maximum_attempts,
       policy.retry_interval_seconds, policy.failure_notification_enabled,
       policy.configured_by
FROM plugin_idc_core_service_entitlement service
LEFT JOIN plugin_idc_core_service_renewal_policy policy
    ON policy.tenant_id = service.tenant_id
   AND policy.policy_no = service.current_renewal_policy_no
   AND policy.policy_version = service.current_renewal_policy_version
   AND policy.entitlement_no = service.entitlement_no
WHERE service.tenant_id = 0 AND service.entitlement_no = ? AND service.customer_no = ?
FOR UPDATE OF service`, entitlementNo, customerNo); err != nil {
		return nil, gerror.Wrap(err, "lock customer renewal policy entitlement")
	}
	return row, nil
}

func samePolicyInput(row *policyEntitlementRow, configuredBy int, input PolicyInput) bool {
	return row != nil && row.PolicyNo != nil && row.PolicyVersion != nil &&
		row.AutoRenewEnabled != nil && *row.AutoRenewEnabled == input.AutoRenewEnabled &&
		row.RenewalCycles != nil && *row.RenewalCycles == input.RenewalCycles &&
		row.LeadSeconds != nil && *row.LeadSeconds == input.LeadSeconds &&
		row.MaximumAttempts != nil && *row.MaximumAttempts == input.MaximumAttempts &&
		row.RetryIntervalSeconds != nil && *row.RetryIntervalSeconds == input.RetryIntervalSeconds &&
		row.FailureNotification != nil &&
		*row.FailureNotification == input.FailureNotificationEnabled &&
		row.ConfiguredBy != nil && *row.ConfiguredBy == configuredBy
}

func projectPolicy(row *policyEntitlementRow, reused bool) *Policy {
	if row == nil || row.PolicyNo == nil || row.PolicyVersion == nil ||
		row.AutoRenewEnabled == nil || row.RenewalCycles == nil || row.LeadSeconds == nil ||
		row.MaximumAttempts == nil || row.RetryIntervalSeconds == nil ||
		row.FailureNotification == nil {
		return nil
	}
	return &Policy{
		PolicyNo: *row.PolicyNo, EntitlementNo: row.EntitlementNo,
		PolicyVersion: *row.PolicyVersion, EntitlementVersion: row.EntitlementVersion,
		AutoRenewEnabled: *row.AutoRenewEnabled, RenewalCycles: *row.RenewalCycles,
		LeadSeconds: *row.LeadSeconds, MaximumAttempts: *row.MaximumAttempts,
		RetryIntervalSeconds:       *row.RetryIntervalSeconds,
		FailureNotificationEnabled: *row.FailureNotification, Reused: reused,
	}
}
