// This file validates the exact frozen fulfillment policy during publication.

package productautomation

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// validateFrozenFulfillmentPolicy is called while the draft version and its
// optional automation binding are locked. It deliberately rebuilds the policy
// from the exact provider release instead of trusting merchant-authored data.
func (s *serviceImpl) validateFrozenFulfillmentPolicy(
	ctx context.Context,
	versionNo string,
	exact *capmodel.BusinessExtensionProviderEvidence,
) error {
	var stored catalog.FulfillmentPolicyEvidence
	if err := s.database.GetScan(ctx, &stored, `SELECT strategy_code,granularity_code,
		ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest
		FROM plugin_idc_core_catalog_product_version_fulfillment_policy
		WHERE tenant_id=? AND version_no=? FOR UPDATE`, idccap.DefaultTenantID, versionNo); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return gerror.Wrap(err, "load frozen fulfillment policy for publication")
	}
	if stored.StrategyCode == "" {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}

	var contract *catalog.FrozenFulfillmentContract
	if stored.StrategyCode != catalog.FulfillmentStrategyManual {
		if exact == nil {
			return bizerr.NewCode(catalog.CodeFulfillmentPolicyBindingRequired)
		}
		declaration, err := productfulfillmentcap.ParseProviderDeclaration(
			exact.ProviderMetadata,
			exact.AuthorizedMethods,
		)
		if err != nil {
			return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
		}
		renewalMode, modeErr := productfulfillmentcap.RenewalMode(declaration)
		if modeErr != nil {
			return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
		}
		contract = &catalog.FrozenFulfillmentContract{
			TrialPauseResume:  declaration.TrialPauseResume,
			Capability:        fulfillmentCapability(declaration),
			Granularity:       declaration.Granularity,
			OngoingManagement: declaration.OngoingManagement,
			RenewalMode:       renewalMode, RenewalMethod: declaration.MethodRoles.Renewal,
			EntitlementOwnerID: declaration.EntitlementOwnerID,
			ProvisionMethod:    declaration.MethodRoles.Provision,
			QueryMethod:        declaration.MethodRoles.Query,
			DeclarationDigest:  exact.DeclarationDigest,
		}
		if err = s.applyImportedFulfillmentContract(ctx, versionNo, exact, contract); err != nil {
			return err
		}
	}
	if err := catalog.ValidateFulfillmentPolicyEvidence(stored, contract); err != nil {
		return err
	}
	return s.validateTrialFulfillment(ctx, versionNo, stored.StrategyCode, contract)
}

// clearDraftFulfillmentPolicy makes a changed or removed binding require one
// explicit merchant choice again. Keeping the old digest would make the detail
// page stale and could accidentally reuse another provider's execution roles.
func (s *serviceImpl) clearDraftFulfillmentPolicy(ctx context.Context, versionNo string) error {
	if _, err := s.database.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_policy
		WHERE tenant_id=? AND version_no=?`, idccap.DefaultTenantID, versionNo); err != nil {
		return gerror.Wrap(err, "clear fulfillment policy after binding change")
	}
	return nil
}

const trialRuleStatusActive = "active"

// validateTrialFulfillment rejects unsuspendable trials before publication. A
// single indexed version lookup checks all active trial rules, including imports.
func (s *serviceImpl) validateTrialFulfillment(ctx context.Context, versionNo, strategy string, contract *catalog.FrozenFulfillmentContract) error {
	if strategy != catalog.FulfillmentStrategyManual && contract != nil && contract.TrialPauseResume {
		return nil
	}
	count, err := s.database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_billing_rule
        WHERE tenant_id=? AND version_no=? AND charge_type_code=? AND status_code=? LIMIT 1`,
		idccap.DefaultTenantID, versionNo, catalog.ChargeTypeTrial, trialRuleStatusActive)
	if err != nil {
		return gerror.Wrap(err, "check trial fulfillment capability")
	}
	if count > 0 {
		return bizerr.NewCode(catalog.CodeTrialFulfillmentUnavailable)
	}
	return nil
}
