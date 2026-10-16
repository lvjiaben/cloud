// This file owns the merchant-facing product-version fulfillment policy while
// keeping plugin method names and execution granularity out of management DTOs.

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

const (
	FulfillmentStrategyManual                 = "manual"
	FulfillmentStrategyApprovalThenPlugin     = "approval_then_plugin"
	FulfillmentStrategyPaymentConfirmedPlugin = "payment_confirmed_plugin"
	fulfillmentPolicyMaximumAttempts          = 10
	fulfillmentPolicyRetryBaseSeconds         = 30
)

var (
	fulfillmentPolicyDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	fulfillmentPolicyMethodPattern = regexp.MustCompile(`^[a-z][a-z0-9]*([._-][a-z0-9]+)*$`)
	entitlementOwnerIDPattern      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// FrozenFulfillmentContract is trusted historical evidence resolved from the
// exact bound plugin release. Technical fields are persisted internally and
// must never be accepted from merchant-facing requests.
type FrozenFulfillmentContract struct {
	TrialPauseResume   bool
	Capability         *capmodel.LocalizedText
	Granularity        string
	OngoingManagement  bool
	RenewalMode        string
	RenewalMethod      string
	EntitlementOwnerID string
	ProvisionMethod    string
	QueryMethod        string
	DeclarationDigest  string
}

// FulfillmentPolicyWriteAuthorization carries trusted actor and release
// evidence after the exact binding-write authorization has succeeded.
type FulfillmentPolicyWriteAuthorization struct {
	ActorUserID int
	Contract    *FrozenFulfillmentContract
}

// FulfillmentContractReader is implemented by product automation so catalog
// never guesses plugin semantics from method names or the current manifest.
type FulfillmentContractReader interface {
	AuthorizeFulfillmentPolicyWrite(
		ctx context.Context,
		versionNo string,
	) (FulfillmentPolicyWriteAuthorization, error)
	GetFrozenFulfillmentContract(
		ctx context.Context,
		versionNo string,
	) (*FrozenFulfillmentContract, error)
}

// FulfillmentPolicyUpdateInput contains the only merchant-authored choice.
// Execution granularity, method names, and retry settings are never accepted.
type FulfillmentPolicyUpdateInput struct {
	VersionNo    string
	StrategyCode string
}

// FulfillmentPolicyEvidence is the immutable internal policy projection.
// Only StrategyCode is suitable for a management request or response.
type FulfillmentPolicyEvidence struct {
	StrategyCode       string
	GranularityCode    string
	OngoingManagement  bool
	RenewalMode        string `orm:"renewal_mode_code"`
	RenewalMethod      string
	EntitlementOwnerID string
	ProvisionMethod    string
	QueryMethod        string
	MaximumAttempts    int
	RetryBaseSeconds   int
	PolicyDigest       string
}

type fulfillmentPolicyDigestInput struct {
	StrategyCode       string `json:"strategyCode"`
	GranularityCode    string `json:"granularityCode"`
	OngoingManagement  bool   `json:"ongoingManagement"`
	RenewalMode        string `json:"renewalMode"`
	RenewalMethod      string `json:"renewalMethod"`
	EntitlementOwnerID string `json:"entitlementOwnerId"`
	ProvisionMethod    string `json:"provisionMethod"`
	QueryMethod        string `json:"queryMethod"`
	DeclarationDigest  string `json:"declarationDigest"`
	MaximumAttempts    int    `json:"maximumAttempts"`
	RetryBaseSeconds   int    `json:"retryBaseSeconds"`
}

// BuildFulfillmentPolicyEvidence is the single canonical digest algorithm used
// by management writes and publication admission.
func BuildFulfillmentPolicyEvidence(
	strategy string,
	contract *FrozenFulfillmentContract,
) (FulfillmentPolicyEvidence, error) {
	strategy = strings.TrimSpace(strategy)
	input := fulfillmentPolicyDigestInput{
		StrategyCode: strategy, MaximumAttempts: fulfillmentPolicyMaximumAttempts,
		RetryBaseSeconds: fulfillmentPolicyRetryBaseSeconds,
		RenewalMode:      productfulfillmentcap.RenewalModeUnavailable,
	}
	switch strategy {
	case FulfillmentStrategyManual:
		input.GranularityCode = productfulfillmentcap.GranularityPerOrderItem
	case FulfillmentStrategyApprovalThenPlugin, FulfillmentStrategyPaymentConfirmedPlugin:
		if contract == nil {
			return FulfillmentPolicyEvidence{}, bizerr.NewCode(CodeFulfillmentPolicyBindingRequired)
		}
		if !validFrozenFulfillmentContract(contract) {
			return FulfillmentPolicyEvidence{}, bizerr.NewCode(CodeFulfillmentPolicyContractChanged)
		}
		input.GranularityCode = contract.Granularity
		input.OngoingManagement = contract.OngoingManagement
		input.RenewalMode = contract.RenewalMode
		input.RenewalMethod = contract.RenewalMethod
		input.EntitlementOwnerID = contract.EntitlementOwnerID
		input.ProvisionMethod = contract.ProvisionMethod
		input.QueryMethod = contract.QueryMethod
		input.DeclarationDigest = contract.DeclarationDigest
	default:
		return FulfillmentPolicyEvidence{}, bizerr.NewCode(CodeManagementInvalid)
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return FulfillmentPolicyEvidence{}, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	digest := sha256.Sum256(encoded)
	return FulfillmentPolicyEvidence{
		StrategyCode: input.StrategyCode, GranularityCode: input.GranularityCode,
		OngoingManagement: input.OngoingManagement,
		RenewalMode:       input.RenewalMode, RenewalMethod: input.RenewalMethod,
		EntitlementOwnerID: input.EntitlementOwnerID,
		ProvisionMethod:    input.ProvisionMethod, QueryMethod: input.QueryMethod,
		MaximumAttempts: input.MaximumAttempts, RetryBaseSeconds: input.RetryBaseSeconds,
		PolicyDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func validFrozenFulfillmentContract(contract *FrozenFulfillmentContract) bool {
	if contract == nil || contract.Capability == nil ||
		strings.TrimSpace(contract.Capability.ZhCN) == "" || strings.TrimSpace(contract.Capability.EnUS) == "" ||
		!fulfillmentPolicyDigestPattern.MatchString(contract.DeclarationDigest) ||
		(contract.Granularity != productfulfillmentcap.GranularityPerOrderItem &&
			contract.Granularity != productfulfillmentcap.GranularityPerQuantityUnit) ||
		!validFrozenRenewalContract(contract) ||
		(contract.OngoingManagement && !entitlementOwnerIDPattern.MatchString(contract.EntitlementOwnerID)) ||
		(!contract.OngoingManagement && contract.EntitlementOwnerID != "") {
		return false
	}
	return fulfillmentPolicyMethodPattern.MatchString(contract.ProvisionMethod) &&
		fulfillmentPolicyMethodPattern.MatchString(contract.QueryMethod) && contract.ProvisionMethod != contract.QueryMethod
}

func validFrozenRenewalContract(contract *FrozenFulfillmentContract) bool {
	switch contract.RenewalMode {
	case productfulfillmentcap.RenewalModeProviderConfirmed:
		return contract.OngoingManagement && fulfillmentPolicyMethodPattern.MatchString(contract.RenewalMethod) &&
			contract.RenewalMethod != contract.ProvisionMethod && contract.RenewalMethod != contract.QueryMethod
	case productfulfillmentcap.RenewalModeLocalEntitlement:
		return contract.OngoingManagement && contract.RenewalMethod == ""
	case productfulfillmentcap.RenewalModeUnavailable:
		return contract.RenewalMethod == ""
	default:
		return false
	}
}

// ValidateFulfillmentPolicyEvidence proves that one stored row still matches
// the exact bound release contract.
func ValidateFulfillmentPolicyEvidence(
	stored FulfillmentPolicyEvidence,
	contract *FrozenFulfillmentContract,
) error {
	expected, err := BuildFulfillmentPolicyEvidence(stored.StrategyCode, contract)
	if err != nil {
		return err
	}
	if stored != expected {
		return bizerr.NewCode(CodeFulfillmentPolicyContractChanged)
	}
	return nil
}

// SaveFulfillmentPolicy persists one draft-only strategy from trusted
// authorization and exact release evidence.
func SaveFulfillmentPolicy(
	ctx context.Context,
	input FulfillmentPolicyUpdateInput,
	authorization FulfillmentPolicyWriteAuthorization,
) (*FulfillmentPolicyEvidence, error) {
	if !versionNumberPattern.MatchString(input.VersionNo) || authorization.ActorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	evidence, err := BuildFulfillmentPolicyEvidence(input.StrategyCode, authorization.Contract)
	if err != nil {
		return nil, err
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var version struct {
			Status string `orm:"status_code"`
		}
		if scanErr := tx.GetScan(&version, `SELECT status_code
			FROM plugin_idc_core_catalog_product_version
			WHERE tenant_id=? AND version_no=? FOR UPDATE`, idccap.DefaultTenantID, input.VersionNo); scanErr != nil {
			return gerror.Wrap(scanErr, "lock fulfillment policy product version")
		}
		if version.Status == "" {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if version.Status != catalogStatusDraft {
			return bizerr.NewCode(CodeFulfillmentPolicyImmutable)
		}
		if evidence.StrategyCode != FulfillmentStrategyManual {
			value, scanErr := tx.GetValue(`SELECT declaration_digest
				FROM plugin_idc_core_catalog_product_version_fulfillment_binding
				WHERE tenant_id=? AND version_no=?`, idccap.DefaultTenantID, input.VersionNo)
			if scanErr != nil {
				return gerror.Wrap(scanErr, "load fulfillment policy binding")
			}
			declarationDigest := value.String()
			if authorization.Contract == nil || declarationDigest != authorization.Contract.DeclarationDigest {
				return bizerr.NewCode(CodeFulfillmentPolicyContractChanged)
			}
		}
		result, execErr := tx.Exec(`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
			(tenant_id,version_no,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,provision_method,query_method,
			 maximum_attempts,retry_base_seconds,policy_digest,created_by,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
			ON CONFLICT (version_no) DO UPDATE SET
			 strategy_code=EXCLUDED.strategy_code, granularity_code=EXCLUDED.granularity_code,
			 ongoing_management=EXCLUDED.ongoing_management,
			 renewal_mode_code=EXCLUDED.renewal_mode_code, renewal_method=EXCLUDED.renewal_method,
			 entitlement_owner_id=EXCLUDED.entitlement_owner_id,
			 provision_method=EXCLUDED.provision_method, query_method=EXCLUDED.query_method,
			 maximum_attempts=EXCLUDED.maximum_attempts, retry_base_seconds=EXCLUDED.retry_base_seconds,
			 policy_digest=EXCLUDED.policy_digest, created_by=EXCLUDED.created_by, created_at=CURRENT_TIMESTAMP`,
			idccap.DefaultTenantID, input.VersionNo, evidence.StrategyCode, evidence.GranularityCode,
			evidence.OngoingManagement, evidence.RenewalMode, evidence.RenewalMethod, evidence.EntitlementOwnerID,
			evidence.ProvisionMethod, evidence.QueryMethod, evidence.MaximumAttempts,
			evidence.RetryBaseSeconds, evidence.PolicyDigest, authorization.ActorUserID)
		if execErr != nil {
			return gerror.Wrap(execErr, "save fulfillment policy")
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return gerror.New("fulfillment policy write did not affect one row")
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeManagementInvalid) || bizerr.Is(err, CodeManagementConflict) ||
			bizerr.Is(err, CodeManagementNotAccessible) ||
			bizerr.Is(err, CodeFulfillmentPolicyBindingRequired) ||
			bizerr.Is(err, CodeFulfillmentPolicyContractChanged) ||
			bizerr.Is(err, CodeFulfillmentPolicyImmutable) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return &evidence, nil
}
