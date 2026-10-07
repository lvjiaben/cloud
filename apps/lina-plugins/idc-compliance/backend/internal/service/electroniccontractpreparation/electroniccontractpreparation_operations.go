// This file implements immutable replacement previews, contract reissues, and terminations.

package electroniccontractpreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

var reissuableContractStates = map[string]struct{}{
	"rejected": {}, "expired": {}, "revoked": {}, "failed": {},
}

type replacementRow struct {
	ReplacementNo     string    `orm:"replacement_no"`
	OldContractNo     string    `orm:"old_contract_no"`
	NewContractNo     string    `orm:"new_contract_no"`
	IdempotencySHA256 string    `orm:"idempotency_sha256"`
	CreatedAt         time.Time `orm:"created_at"`
}

type terminationSnapshot struct {
	ContractNo                string    `json:"contractNo"`
	OriginalFlowNo            string    `json:"originalFlowNo"`
	OriginalRouteNo           string    `json:"originalRouteNo"`
	Reason                    string    `json:"reason"`
	RemainInForceItem         string    `json:"remainInForceItem,omitempty"`
	OriginalExpenseSettlement string    `json:"originalExpenseSettlement,omitempty"`
	OriginalOtherSettlement   string    `json:"originalOtherSettlement,omitempty"`
	OtherDeals                string    `json:"otherDeals,omitempty"`
	CreatedAt                 time.Time `json:"createdAt"`
	ExpiresAt                 time.Time `json:"expiresAt"`
}

type replacementSemanticSnapshot struct {
	OrderNo        string
	Template       contractcap.TemplateSnapshot
	Preview        contractcap.PreviewVariables
	CustomerSigner customerSignerSnapshot
	ProviderLaunch providerLaunchSnapshot
}

func (s *serviceImpl) PrepareReplacement(
	ctx context.Context,
	input PrepareReplacementInput,
) (result *PreparedContract, resultErr error) {
	defer func() {
		resultErr = normalizeContractManagementError(resultErr)
	}()
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.SignerNo = strings.TrimSpace(input.SignerNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!validBusinessNo(input.ContractNo, 64) || !validBusinessNo(input.TemplateNo, 64) ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractReissue, TargetNo: input.ContractNo,
	}); err != nil {
		return nil, err
	}
	oldContract, err := s.loadByContractNo(ctx, input.ContractNo)
	if err != nil || validateStoredRow(oldContract) != nil {
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeNotAccessible)
		}
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if _, allowed := reissuableContractStates[oldContract.StatusCode]; !allowed {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	oldSemantic, err := s.loadReplacementSemantic(ctx, oldContract)
	if err != nil {
		return nil, err
	}
	idempotencyDigest := digestText(
		"idc/contract-replacement-idempotency/v1\x00", input.IdempotencyKey,
	)
	state := new(preparationState)
	defer state.clear()
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return s.prepareReplacementInTransaction(
			ctx, tx, state, oldContract, oldSemantic, input, idempotencyDigest,
		)
	})
	if err != nil {
		if state.prepared != nil {
			state.prepared.Clear()
		}
		return nil, err
	}
	if state.prepared != nil {
		return state.prepared, nil
	}
	if state.persisted == nil {
		return nil, gerror.New("contract replacement preview was not persisted")
	}
	return s.projectPersisted(ctx, state.persisted, true)
}

// GetReplacementOptions returns only current safe choices and never creates a preview or Tencent flow.
func (s *serviceImpl) GetReplacementOptions(
	ctx context.Context,
	contractNo string,
) (*ReplacementOptions, error) {
	contractNo = strings.TrimSpace(contractNo)
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(contractNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractReissue, TargetNo: contractNo,
	}); err != nil {
		return nil, err
	}
	oldContract, err := s.loadByContractNo(ctx, contractNo)
	if err != nil || validateStoredRow(oldContract) != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if _, allowed := reissuableContractStates[oldContract.StatusCode]; !allowed {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	order, err := s.runtime.Orders.PrepareOptionalOrder(ctx, oldContract.OrderNo)
	if oldContract.RequiredForFulfillment {
		order, err = s.runtime.Orders.PrepareOrder(ctx, oldContract.OrderNo)
	}
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if order == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer order.Clear()
	selection := contracttemplatecap.Selection{
		Level:        contracttemplatecap.SelectionLevelOrder,
		CustomerType: contracttemplatecap.CustomerType(order.Customer.CustomerType),
	}
	available, err := s.templates.ListAvailable(ctx, selection)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	templates := make([]ManualOptionalTemplate, 0, len(available))
	for _, template := range available {
		if err = contracttemplatecap.ValidateTemplate(template, selection); err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
		templates = append(templates, ManualOptionalTemplate{
			TemplateNo: template.TemplateNo, VersionNo: template.VersionNo, Name: template.Name,
			ScopeCode: template.ScopeCode, SigningOrderCode: template.SigningOrderCode,
			LanguageCode: template.LanguageCode,
		})
	}
	signers := []ReplacementSigner{}
	if order.Customer.CustomerType == contractcap.CustomerTypeEnterprise {
		if s.signers == nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		candidates, candidateErr := s.signers.ListReissueCandidates(ctx, order.Customer.CustomerNo, contractNo)
		if candidateErr != nil {
			return nil, candidateErr
		}
		signers = make([]ReplacementSigner, 0, len(candidates))
		for _, candidate := range candidates {
			signers = append(signers, ReplacementSigner{
				SignerNo: candidate.SignerNo, MaskedName: candidate.MaskedName, MaskedPhone: candidate.MaskedPhone,
			})
		}
	}
	return &ReplacementOptions{
		ContractNo: contractNo, OrderNo: oldContract.OrderNo, CustomerNo: order.Customer.CustomerNo,
		CustomerType: order.Customer.CustomerType, Templates: templates, Signers: signers,
	}, nil
}

func (s *serviceImpl) prepareReplacementInTransaction(
	ctx context.Context,
	tx gdb.TX,
	state *preparationState,
	oldContract *contractRow,
	oldSemantic string,
	input PrepareReplacementInput,
	idempotencyDigest string,
) error {
	lockKeys := []string{
		"contract-replacement-idempotency:" + idempotencyDigest,
		"contract-replacement-old:" + oldContract.ContractNo,
	}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err := tx.GetValue("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
			return gerror.Wrap(err, "lock contract replacement preview")
		}
	}
	var existing []*replacementRow
	if err := tx.Ctx(ctx).GetScan(&existing, `
SELECT replacement_no, old_contract_no, new_contract_no, idempotency_sha256, created_at
FROM plugin_idc_compliance_contract_replacement
WHERE tenant_id = ? AND (old_contract_no = ? OR idempotency_sha256 = ?)
ORDER BY CASE WHEN idempotency_sha256 = ? THEN 0 ELSE 1 END, id ASC
LIMIT 2 FOR UPDATE`, idccap.DefaultTenantID, oldContract.ContractNo,
		idempotencyDigest, idempotencyDigest); err != nil {
		return gerror.Wrap(err, "load contract replacement preview")
	}
	if len(existing) == 1 && existing[0] != nil &&
		existing[0].OldContractNo == oldContract.ContractNo &&
		existing[0].IdempotencySHA256 == idempotencyDigest {
		persisted, err := loadContractRowTX(ctx, tx, existing[0].NewContractNo)
		if err != nil {
			return err
		}
		state.persisted = persisted
		return nil
	}
	if len(existing) != 0 {
		return gerror.New("contract replacement already exists or the idempotency key is in use")
	}
	current, err := lockContractRow(ctx, tx, oldContract.ContractNo)
	if err != nil {
		return err
	}
	if _, allowed := reissuableContractStates[current.StatusCode]; !allowed {
		return gerror.New("contract state changed before replacement preview creation")
	}
	state.order, err = s.loadPreparationOrder(
		ctx, current.OrderNo, input.TemplateNo, current.RequiredForFulfillment,
	)
	if err != nil {
		return err
	}
	if current.RequiredForFulfillment {
		if err = s.applySelectedTemplate(ctx, state.order, input.TemplateNo); err != nil {
			return err
		}
	}
	databaseTime, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return gerror.Wrap(err, "load contract replacement preview time")
	}
	contractNo, err := newContractNo(s.random)
	if err != nil {
		return err
	}
	preview, err := state.order.BuildPreviewVariables(contractNo)
	if err != nil {
		return err
	}
	defer preview.Clear()
	customerSigner, err := s.resolveCustomerSigner(ctx, state.order, input.SignerNo)
	if err != nil {
		return err
	}
	defer customerSigner.Clear()
	providerLaunch, err := loadProviderLaunchSnapshot(ctx, tx, state.order)
	if err != nil {
		return err
	}
	defer providerLaunch.Clear()
	var digests snapshotDigests
	state.bundle, digests, state.payload, err = newSnapshotBundle(
		s.random, contractNo, databaseTime.Time().UTC(), state.order, preview, customerSigner, providerLaunch,
	)
	if err != nil {
		return err
	}
	newSemantic, err := replacementSemanticDigest(state.bundle)
	if err != nil {
		return err
	}
	if secureDigestEqual(oldSemantic, newSemantic) {
		return bizerr.NewCode(CodeNotSignable)
	}
	fingerprint := digestText(
		"idc/contract-replacement-fingerprint/v1\x00",
		oldContract.ContractNo+"\x00"+newSemantic,
	)
	created, err := s.manager.Create(
		ctx, "contract-replacement-snapshot:"+idempotencyDigest, bundleProducer(state.payload),
	)
	if err != nil {
		return gerror.Wrap(err, "seal contract replacement preview")
	}
	if err = s.persistReplacementSnapshot(
		ctx, tx, state, created, digests, newSemantic, idempotencyDigest, fingerprint,
	); err != nil {
		return err
	}
	if state.persisted == nil {
		return gerror.New("persisted replacement contract is unavailable")
	}
	replacementNo, err := newReplacementNo(s.random)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`
INSERT INTO plugin_idc_compliance_contract_replacement
    (tenant_id, replacement_no, old_contract_no, new_contract_no, idempotency_sha256, created_at)
VALUES (?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, replacementNo,
		oldContract.ContractNo, state.persisted.ContractNo, idempotencyDigest,
		databaseTime.Time().UTC()); err != nil {
		return gerror.Wrap(err, "insert immutable contract replacement relation")
	}
	return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.contract.replacement.prepare",
		IdempotencyKey:   idempotencyDigest, OperationCode: "compliance.contract.replacement.prepare",
		TargetType: "contract", TargetNo: state.persisted.ContractNo,
		CustomerNo: state.persisted.CustomerNo, ResultCode: "prepared",
		ReasonCode: "changed_contract_replacement", RiskCode: "high",
		CorrelationNo: replacementNo,
		AfterSnapshot: map[string]any{
			"state": "pending_launch", "old_contract_no": oldContract.ContractNo,
			"new_contract_no": state.persisted.ContractNo,
		},
	})
}

func (s *serviceImpl) persistReplacementSnapshot(
	ctx context.Context,
	tx gdb.TX,
	state *preparationState,
	created secretcap.CreateResult,
	digests snapshotDigests,
	expectedSemantic string,
	idempotencyDigest string,
	fingerprint string,
) error {
	if created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		return gerror.New("sealed contract replacement preview metadata is invalid")
	}
	reference := created.Handle.OpaqueRef()
	if reference == "" {
		return gerror.New("contract replacement snapshot reference is unavailable")
	}
	if created.Reused {
		state.bundle.Clear()
		state.bundle = nil
		return s.withSnapshotBundle(ctx, created.Handle, "compliance.contract-replacement-recovery", nil,
			func(recovered *snapshotBundle) error {
				actualSemantic, err := replacementSemanticDigest(recovered)
				if err != nil || !secureDigestEqual(actualSemantic, expectedSemantic) {
					return gerror.New("recovered contract replacement preview conflicts with this request")
				}
				recoveredDigests, err := digestBundle(recovered)
				if err != nil {
					return err
				}
				if err = s.persistPrepared(ctx, tx, recovered, recoveredDigests, reference,
					idempotencyDigest, fingerprint); err != nil {
					return err
				}
				state.persisted = rowFromBundle(
					recovered, recoveredDigests, reference, idempotencyDigest, fingerprint,
				)
				state.prepared = projectPrepared(recovered, true)
				return nil
			})
	}
	if state.bundle == nil {
		return gerror.New("contract replacement preview bundle is unavailable")
	}
	if err := s.persistPrepared(
		ctx, tx, state.bundle, digests, reference, idempotencyDigest, fingerprint,
	); err != nil {
		return err
	}
	state.persisted = rowFromBundle(
		state.bundle, digests, reference, idempotencyDigest, fingerprint,
	)
	state.prepared = projectPrepared(state.bundle, false)
	return nil
}

func (s *serviceImpl) loadReplacementSemantic(
	ctx context.Context,
	row *contractRow,
) (string, error) {
	handle, err := secretcap.NewHandle(row.SnapshotKeyRef)
	if err != nil {
		return "", gerror.New("contract replacement source snapshot reference is invalid")
	}
	expected := snapshotDigests{
		Variables: row.VariableSnapshotSHA256,
		Signers:   row.SignerSnapshotSHA256,
		Order:     row.OrderSnapshotSHA256,
	}
	var semantic string
	err = s.withSnapshotBundle(ctx, handle, "compliance.contract-replacement-compare", &expected,
		func(bundle *snapshotBundle) error {
			if err := validateBundleRow(bundle, row); err != nil {
				return err
			}
			semantic, err = replacementSemanticDigest(bundle)
			return err
		})
	return semantic, err
}

func replacementSemanticDigest(bundle *snapshotBundle) (string, error) {
	if bundle == nil {
		return "", gerror.New("contract replacement comparison snapshot is unavailable")
	}
	preview := clonePreviewVariables(&bundle.Preview)
	preview.ContractNo = ""
	defer preview.Clear()
	signer := cloneCustomerSigner(&bundle.CustomerSigner)
	defer signer.Clear()
	provider := bundle.ProviderLaunch.Clone()
	defer provider.Clear()
	payload, err := json.Marshal(replacementSemanticSnapshot{
		OrderNo: bundle.Order.OrderNo, Template: bundle.Order.Template,
		Preview: preview, CustomerSigner: signer, ProviderLaunch: provider,
	})
	if err != nil || len(payload) == 0 {
		clear(payload)
		return "", gerror.New("contract replacement comparison snapshot is invalid")
	}
	defer clear(payload)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func loadContractRowTX(ctx context.Context, tx gdb.TX, contractNo string) (*contractRow, error) {
	var rows []*contractRow
	if err := tx.Ctx(ctx).GetScan(&rows, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ? LIMIT 2`, idccap.DefaultTenantID, contractNo); err != nil {
		return nil, gerror.Wrap(err, "load replacement contract preview")
	}
	if len(rows) != 1 || validateStoredRow(rows[0]) != nil {
		return nil, gerror.New("replacement contract preview is unavailable")
	}
	return rows[0], nil
}

func (s *serviceImpl) SubmitReissue(
	ctx context.Context,
	input ReissueSigningInput,
) (result *SubmittedSigning, resultErr error) {
	defer func() {
		resultErr = normalizeContractManagementError(resultErr)
	}()
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if s == nil || s.gateway == nil || ctx == nil || ctx.Err() != nil ||
		!validBusinessNo(input.ContractNo, 64) || input.ExpectedVersion < 1 ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractReissue, TargetNo: input.ContractNo,
	}); err != nil {
		return nil, err
	}
	idempotencyDigest := digestText("idc/contract-reissue-idempotency/v1\x00", input.IdempotencyKey)
	if replay, found, err := s.loadReissueReplay(ctx, input.ContractNo, idempotencyDigest); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}
	row, err := s.loadByContractNo(ctx, input.ContractNo)
	if err != nil || validateStoredRow(row) != nil {
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeNotAccessible)
		}
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	databaseTime, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "load contract reissue time")
	}
	now := databaseTime.Time().UTC()
	replacement, err := s.loadIncomingReplacement(ctx, input.ContractNo)
	if err != nil {
		return nil, err
	}
	deadline := now.Add(SigningValidity)
	if replacement != nil {
		deadline = row.SigningExpiresAt
	}
	if !deadline.After(now) {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	request, err := s.buildReissueRequest(ctx, row, deadline)
	if err != nil {
		return nil, err
	}
	requestDigest, err := digestJSON(request)
	if err != nil {
		return nil, err
	}
	flow, version, launchRequired, err := s.reserveReissueFlow(
		ctx, row, replacement, input.ExpectedVersion, idempotencyDigest, requestDigest, deadline,
	)
	if err != nil {
		return nil, err
	}
	if !launchRequired {
		return &SubmittedSigning{ContractNo: row.ContractNo, FlowNo: flow.FlowNo,
			StatusCode: flow.StatusCode, Version: version, Reused: true}, nil
	}
	gatewayResult, gatewayErr := s.gateway.CreateTencentContractFlow(
		ctx, request, "contract-reissue:"+idempotencyDigest,
	)
	resultUnknown := false
	if unknown, ok := gatewayErr.(interface{ IsResultUnknown() bool }); ok {
		resultUnknown = unknown.IsResultUnknown()
	}
	status, version, finishErr := s.finishReissueFlow(
		ctx, flow, requestDigest, gatewayResult.RouteNo, gatewayErr, resultUnknown,
	)
	if finishErr != nil {
		return nil, finishErr
	}
	if gatewayErr != nil {
		return nil, gatewayErr
	}
	return &SubmittedSigning{ContractNo: row.ContractNo, FlowNo: flow.FlowNo,
		StatusCode: status, Version: version, Reused: gatewayResult.Recovered}, nil
}

func (s *serviceImpl) loadReissueReplay(
	ctx context.Context,
	contractNo string,
	idempotencyDigest string,
) (*SubmittedSigning, bool, error) {
	var flows []*contractFlowRow
	if err := s.database.GetScan(ctx, &flows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND purpose_code = 'reissue' AND idempotency_sha256 = ?
LIMIT 2`, idccap.DefaultTenantID, idempotencyDigest); err != nil {
		return nil, false, gerror.Wrap(err, "load contract reissue replay")
	}
	if len(flows) == 0 {
		return nil, false, nil
	}
	if len(flows) != 1 || flows[0] == nil || flows[0].ContractNo != contractNo {
		return nil, false, gerror.New("contract reissue idempotency key conflicts with another request")
	}
	flow := flows[0]
	if flow.StatusCode != "signing" && flow.StatusCode != "result_unknown" {
		return nil, false, gerror.New("existing contract reissue cannot be submitted again")
	}
	version, err := s.database.GetValue(ctx, `SELECT version
FROM plugin_idc_compliance_contract WHERE tenant_id = ? AND contract_no = ?`,
		idccap.DefaultTenantID, contractNo)
	if err != nil || version.Int64() < 1 {
		return nil, false, gerror.New("reissued contract version is unavailable")
	}
	return &SubmittedSigning{ContractNo: contractNo, FlowNo: flow.FlowNo,
		StatusCode: flow.StatusCode, Version: version.Int64(), Reused: true}, true, nil
}

func (s *serviceImpl) buildReissueRequest(
	ctx context.Context,
	row *contractRow,
	deadline time.Time,
) (processauth.TencentContractFlowRequest, error) {
	handle, err := secretcap.NewHandle(row.SnapshotKeyRef)
	if err != nil {
		return processauth.TencentContractFlowRequest{}, err
	}
	expected := snapshotDigests{Variables: row.VariableSnapshotSHA256,
		Signers: row.SignerSnapshotSHA256, Order: row.OrderSnapshotSHA256}
	var request processauth.TencentContractFlowRequest
	err = s.withSnapshotBundle(ctx, handle, "compliance.contract-reissue-submit", &expected,
		func(bundle *snapshotBundle) error {
			if err := validateBundleRow(bundle, row); err != nil {
				return err
			}
			var buildErr error
			request, buildErr = s.buildContractFlowRequestWithDeadline(bundle, deadline)
			return buildErr
		})
	return request, err
}

func (s *serviceImpl) loadIncomingReplacement(
	ctx context.Context,
	contractNo string,
) (*replacementRow, error) {
	var rows []*replacementRow
	if err := s.database.GetScan(ctx, &rows, `
SELECT replacement_no, old_contract_no, new_contract_no, idempotency_sha256, created_at
FROM plugin_idc_compliance_contract_replacement
WHERE tenant_id = ? AND new_contract_no = ?
LIMIT 2`, idccap.DefaultTenantID, contractNo); err != nil {
		return nil, gerror.Wrap(err, "load contract replacement")
	}
	if len(rows) > 1 || len(rows) == 1 && rows[0] == nil {
		return nil, gerror.New("contract replacement is ambiguous")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (s *serviceImpl) reserveReissueFlow(
	ctx context.Context,
	loaded *contractRow,
	replacement *replacementRow,
	expectedVersion int64,
	idempotencyDigest string,
	requestDigest string,
	deadline time.Time,
) (*contractFlowRow, int64, bool, error) {
	var flow *contractFlowRow
	var version int64
	launchRequired := false
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, key := range []string{"contract-flow:" + loaded.ContractNo, "contract-flow-idem:" + idempotencyDigest} {
			if _, err := tx.GetValue("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
				return gerror.Wrap(err, "lock contract reissue")
			}
		}
		contract, err := lockContractRow(ctx, tx, loaded.ContractNo)
		if err != nil {
			return err
		}
		version = contract.Version
		var existing []*contractFlowRow
		if err = tx.Ctx(ctx).GetScan(&existing, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND idempotency_sha256 = ?
LIMIT 2 FOR UPDATE`, idccap.DefaultTenantID, idempotencyDigest); err != nil {
			return gerror.Wrap(err, "load contract reissue flow")
		}
		if len(existing) == 1 {
			flow = existing[0]
			if flow == nil || flow.ContractNo != contract.ContractNo ||
				flow.RequestSnapshotSHA256 != requestDigest || flow.RequestSnapshotRef != contract.SnapshotKeyRef {
				return gerror.New("contract reissue idempotency key conflicts with another request")
			}
			if flow.StatusCode == "signing" || flow.StatusCode == "result_unknown" {
				return nil
			}
			return gerror.New("existing contract reissue cannot be submitted again")
		}
		if len(existing) > 1 || contract.Version != expectedVersion {
			return gerror.New("contract reissue state changed")
		}
		if replacement == nil {
			if _, ok := reissuableContractStates[contract.StatusCode]; !ok {
				return gerror.New("contract state does not allow an unchanged reissue")
			}
		} else {
			if replacement.NewContractNo != contract.ContractNo || contract.StatusCode != "pending_launch" {
				return gerror.New("replacement contract is unavailable for reissue")
			}
			old, loadErr := lockContractRow(ctx, tx, replacement.OldContractNo)
			if loadErr != nil {
				return loadErr
			}
			if _, ok := reissuableContractStates[old.StatusCode]; !ok || old.OrderNo != contract.OrderNo {
				return gerror.New("replaced contract state changed")
			}
		}
		sequenceValue, err := tx.GetValue(`SELECT COALESCE(MAX(sequence_no), 0) + 1
FROM plugin_idc_compliance_contract_flow WHERE tenant_id = ? AND contract_no = ?`,
			idccap.DefaultTenantID, contract.ContractNo)
		if err != nil || sequenceValue.Int() < 1 {
			return gerror.New("contract reissue sequence is unavailable")
		}
		flowNo, err := newFlowNo(s.random)
		if err != nil {
			return err
		}
		nowValue, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if err != nil {
			return err
		}
		now := nowValue.Time().UTC()
		if !deadline.After(now) {
			return gerror.New("contract reissue deadline has expired")
		}
		if _, err = tx.Exec(`
INSERT INTO plugin_idc_compliance_contract_flow
    (tenant_id, flow_no, contract_no, sequence_no, purpose_code, provider_code,
     idempotency_sha256, request_snapshot_ref, request_snapshot_sha256,
     status_code, available_at, expires_at, version, created_at, updated_at)
VALUES (?, ?, ?, ?, 'reissue', ?, ?, ?, ?, 'submitting', ?, ?, 1, ?, ?)`,
			idccap.DefaultTenantID, flowNo, contract.ContractNo, sequenceValue.Int(),
			ProviderTencentESSBasic, idempotencyDigest, contract.SnapshotKeyRef,
			requestDigest, now, deadline, now, now); err != nil {
			return gerror.Wrap(err, "reserve contract reissue flow")
		}
		result, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract
SET status_code = 'reissuing', version = version + 1, updated_at = ?
WHERE tenant_id = ? AND contract_no = ? AND version = ?`, now,
			idccap.DefaultTenantID, contract.ContractNo, contract.Version)
		if err != nil {
			return gerror.Wrap(err, "reserve contract reissue state")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract reissue state changed")
		}
		version = contract.Version + 1
		flow = &contractFlowRow{FlowNo: flowNo, ContractNo: contract.ContractNo,
			IdempotencySHA256: idempotencyDigest, RequestSnapshotRef: contract.SnapshotKeyRef,
			RequestSnapshotSHA256: requestDigest, StatusCode: "submitting", ExpiresAt: &deadline, Version: 1}
		if err = s.appendOperationAudit(ctx, "reissue", flow, "submitting"); err != nil {
			return err
		}
		launchRequired = true
		return nil
	})
	return flow, version, launchRequired, err
}

func (s *serviceImpl) finishReissueFlow(
	ctx context.Context,
	reserved *contractFlowRow,
	requestDigest string,
	gatewayRouteNo string,
	gatewayErr error,
	resultUnknown bool,
) (string, int64, error) {
	status := "signing"
	contractStatus := "signing"
	failureCode := ""
	if gatewayErr != nil {
		failureCode = boundedFailureCode(gatewayErr.Error())
		if resultUnknown {
			status = "result_unknown"
		} else {
			status = "failed"
			contractStatus = "failed"
		}
	}
	var version int64
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		flow, err := lockOperationFlow(ctx, tx, reserved.FlowNo)
		if err != nil {
			return err
		}
		if flow.ContractNo != reserved.ContractNo || flow.IdempotencySHA256 != reserved.IdempotencySHA256 ||
			flow.RequestSnapshotSHA256 != requestDigest {
			return gerror.New("submitted contract reissue changed unexpectedly")
		}
		if flow.StatusCode == "signing" && gatewayErr == nil {
			value, loadErr := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, reserved.ContractNo)
			version = value.Int64()
			return loadErr
		}
		if flow.StatusCode != "submitting" && flow.StatusCode != "result_unknown" {
			return gerror.New("contract reissue flow is not mutable")
		}
		nowValue, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if err != nil {
			return err
		}
		now := nowValue.Time().UTC()
		var route any
		if gatewayRouteNo != "" {
			route = gatewayRouteNo
		}
		flowResult, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract_flow
SET gateway_route_no = ?, status_code = ?, submitted_at = CASE WHEN ? IN ('signing','result_unknown') THEN COALESCE(submitted_at, ?) ELSE submitted_at END,
    failure_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND flow_no = ? AND version = ?`, route, status, status, now,
			failureCode, now, idccap.DefaultTenantID, flow.FlowNo, flow.Version)
		if err != nil {
			return gerror.Wrap(err, "finish contract reissue flow")
		}
		if affected, affectedErr := flowResult.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract reissue flow update conflicted")
		}
		result, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract
SET status_code = ?, launched_at = CASE WHEN ? = 'signing' THEN COALESCE(launched_at, ?) ELSE launched_at END,
    version = version + 1, updated_at = ?
WHERE tenant_id = ? AND contract_no = ? AND status_code = 'reissuing'`, contractStatus,
			contractStatus, now, now, idccap.DefaultTenantID, flow.ContractNo)
		if err != nil {
			return gerror.Wrap(err, "finish contract reissue state")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract reissue state update conflicted")
		}
		value, err := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, flow.ContractNo)
		if err != nil {
			return err
		}
		version = value.Int64()
		return s.appendOperationAudit(ctx, "reissue", flow, status)
	})
	return status, version, err
}

func (s *serviceImpl) TerminateContract(
	ctx context.Context,
	input TerminateContractInput,
) (result *SubmittedSigning, resultErr error) {
	defer func() {
		resultErr = normalizeContractManagementError(resultErr)
	}()
	if err := normalizeTerminationInput(&input); err != nil {
		return nil, err
	}
	if s == nil || s.gateway == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractTerminate, TargetNo: input.ContractNo,
	}); err != nil {
		return nil, err
	}
	idempotencyDigest := digestText("idc/contract-termination-idempotency/v1\x00", input.IdempotencyKey)
	if replay, found, err := s.loadTerminationReplay(ctx, input, idempotencyDigest); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}
	contract, err := s.loadByContractNo(ctx, input.ContractNo)
	if err != nil || validateStoredRow(contract) != nil || contract.StatusCode != "completed" ||
		contract.Version != input.ExpectedVersion {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	originalFlow, err := s.loadCompletedSigningFlow(ctx, contract.ContractNo)
	if err != nil {
		return nil, err
	}
	databaseTime, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "load contract termination time")
	}
	now := databaseTime.Time().UTC()
	deadline := now.Add(SigningValidity)
	snapshot := terminationSnapshot{
		ContractNo: contract.ContractNo, OriginalFlowNo: originalFlow.FlowNo,
		OriginalRouteNo: strings.TrimSpace(pointerString(originalFlow.GatewayRouteNo)),
		Reason:          input.Reason, RemainInForceItem: input.RemainInForceItem,
		OriginalExpenseSettlement: input.OriginalExpenseSettlement,
		OriginalOtherSettlement:   input.OriginalOtherSettlement, OtherDeals: input.OtherDeals,
		CreatedAt: now, ExpiresAt: deadline,
	}
	payload, err := json.Marshal(snapshot)
	if err != nil || len(payload) == 0 {
		clear(payload)
		return nil, gerror.New("contract termination snapshot is invalid")
	}
	defer clear(payload)
	requestDigest := sha256.Sum256(payload)
	requestSHA256 := hex.EncodeToString(requestDigest[:])
	created, err := s.manager.Create(ctx, "contract-termination:"+idempotencyDigest, bundleProducer(payload))
	if err != nil {
		return nil, gerror.Wrap(err, "seal contract termination snapshot")
	}
	if created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		return nil, gerror.New("sealed contract termination snapshot metadata is invalid")
	}
	requestReference := created.Handle.OpaqueRef()
	if requestReference == "" {
		return nil, gerror.New("contract termination snapshot reference is unavailable")
	}
	if created.Reused {
		recovered, recoveredDigest, recoverErr := s.loadTerminationSnapshot(
			ctx, created.Handle, "compliance.contract-termination-recovery",
		)
		if recoverErr != nil || !sameTerminationRequest(recovered, snapshot) {
			return nil, gerror.New("recovered contract termination snapshot conflicts with this request")
		}
		snapshot = recovered
		requestSHA256 = recoveredDigest
		deadline = snapshot.ExpiresAt
	}
	request := processauth.TencentContractTerminationRequest{
		ContractNo: contract.ContractNo, OriginalRouteNo: snapshot.OriginalRouteNo,
		Reason: snapshot.Reason, RemainInForceItem: snapshot.RemainInForceItem,
		OriginalExpenseSettlement: snapshot.OriginalExpenseSettlement,
		OriginalOtherSettlement:   snapshot.OriginalOtherSettlement,
		OtherDeals:                snapshot.OtherDeals, DeadlineUnix: deadline.Unix(),
	}
	flow, version, launchRequired, err := s.reserveTerminationFlow(
		ctx, contract, originalFlow, input.ExpectedVersion, idempotencyDigest,
		requestReference, requestSHA256, deadline,
	)
	if err != nil {
		return nil, err
	}
	if !launchRequired {
		return &SubmittedSigning{ContractNo: contract.ContractNo, FlowNo: flow.FlowNo,
			StatusCode: flow.StatusCode, Version: version, Reused: true}, nil
	}
	gatewayResult, gatewayErr := s.gateway.CreateTencentContractTermination(
		ctx, request, "contract-termination:"+idempotencyDigest,
	)
	resultUnknown := false
	if unknown, ok := gatewayErr.(interface{ IsResultUnknown() bool }); ok {
		resultUnknown = unknown.IsResultUnknown()
	}
	status, version, finishErr := s.finishTerminationFlow(
		ctx, flow, requestSHA256, gatewayResult.RouteNo, gatewayErr, resultUnknown,
	)
	if finishErr != nil {
		return nil, finishErr
	}
	if gatewayErr != nil {
		return nil, gatewayErr
	}
	return &SubmittedSigning{ContractNo: contract.ContractNo, FlowNo: flow.FlowNo,
		StatusCode: status, Version: version, Reused: gatewayResult.Recovered}, nil
}

func normalizeTerminationInput(input *TerminateContractInput) error {
	if input == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.Reason = strings.TrimSpace(input.Reason)
	input.RemainInForceItem = strings.TrimSpace(input.RemainInForceItem)
	input.OriginalExpenseSettlement = strings.TrimSpace(input.OriginalExpenseSettlement)
	input.OriginalOtherSettlement = strings.TrimSpace(input.OriginalOtherSettlement)
	input.OtherDeals = strings.TrimSpace(input.OtherDeals)
	if !input.Confirmed ||
		!validBusinessNo(input.ContractNo, 64) || input.ExpectedVersion < 1 ||
		!validIdempotencyKey(input.IdempotencyKey) || !validReleaseText(input.Reason, true) ||
		!validReleaseText(input.RemainInForceItem, false) ||
		!validReleaseText(input.OriginalExpenseSettlement, false) ||
		!validReleaseText(input.OriginalOtherSettlement, false) || !validReleaseText(input.OtherDeals, false) {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func normalizeContractManagementError(err error) error {
	if err == nil {
		return nil
	}
	var businessError *bizerr.Error
	if errors.As(err, &businessError) {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}

func (s *serviceImpl) loadTerminationReplay(
	ctx context.Context,
	input TerminateContractInput,
	idempotencyDigest string,
) (*SubmittedSigning, bool, error) {
	var flows []*contractFlowRow
	if err := s.database.GetScan(ctx, &flows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND purpose_code = 'termination' AND idempotency_sha256 = ?
LIMIT 2`, idccap.DefaultTenantID, idempotencyDigest); err != nil {
		return nil, false, gerror.Wrap(err, "load contract termination replay")
	}
	if len(flows) == 0 {
		return nil, false, nil
	}
	if len(flows) != 1 || flows[0] == nil || flows[0].ContractNo != input.ContractNo {
		return nil, false, gerror.New("contract termination idempotency key conflicts with another request")
	}
	flow := flows[0]
	handle, err := secretcap.NewHandle(flow.RequestSnapshotRef)
	if err != nil {
		return nil, false, gerror.New("contract termination replay snapshot is unavailable")
	}
	snapshot, digest, err := s.loadTerminationSnapshot(
		ctx, handle, "compliance.contract-termination-replay",
	)
	if err != nil || !secureDigestEqual(digest, flow.RequestSnapshotSHA256) ||
		!terminationInputMatches(snapshot, input) {
		return nil, false, gerror.New("contract termination idempotency key conflicts with another request")
	}
	if flow.StatusCode != "signing" && flow.StatusCode != "result_unknown" {
		return nil, false, gerror.New("existing contract termination cannot be submitted again")
	}
	version, err := s.database.GetValue(ctx, `SELECT version
FROM plugin_idc_compliance_contract WHERE tenant_id = ? AND contract_no = ?`,
		idccap.DefaultTenantID, input.ContractNo)
	if err != nil || version.Int64() < 1 {
		return nil, false, gerror.New("terminated contract version is unavailable")
	}
	return &SubmittedSigning{ContractNo: input.ContractNo, FlowNo: flow.FlowNo,
		StatusCode: flow.StatusCode, Version: version.Int64(), Reused: true}, true, nil
}

func (s *serviceImpl) loadTerminationSnapshot(
	ctx context.Context,
	handle secretcap.Handle,
	purposeCode string,
) (terminationSnapshot, string, error) {
	purpose, err := secretcap.NewPurpose(purposeCode)
	if err != nil {
		return terminationSnapshot{}, "", err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return terminationSnapshot{}, "", err
	}
	var result terminationSnapshot
	var digest string
	err = s.accessor.Access(ctx, request, func(payload []byte) error {
		if len(payload) == 0 || len(payload) > maximumBundleBytes || json.Unmarshal(payload, &result) != nil ||
			!validTerminationSnapshot(result) {
			return gerror.New("contract termination snapshot is invalid")
		}
		hash := sha256.Sum256(payload)
		digest = hex.EncodeToString(hash[:])
		return nil
	})
	return result, digest, err
}

func validTerminationSnapshot(value terminationSnapshot) bool {
	return validBusinessNo(value.ContractNo, 64) && validBusinessNo(value.OriginalFlowNo, 64) &&
		customerGatewayRoutePattern.MatchString(value.OriginalRouteNo) &&
		validReleaseText(value.Reason, true) && validReleaseText(value.RemainInForceItem, false) &&
		validReleaseText(value.OriginalExpenseSettlement, false) &&
		validReleaseText(value.OriginalOtherSettlement, false) && validReleaseText(value.OtherDeals, false) &&
		!value.CreatedAt.IsZero() && value.ExpiresAt.Equal(value.CreatedAt.Add(SigningValidity))
}

func sameTerminationRequest(left, right terminationSnapshot) bool {
	return left.ContractNo == right.ContractNo && left.OriginalFlowNo == right.OriginalFlowNo &&
		left.OriginalRouteNo == right.OriginalRouteNo && left.Reason == right.Reason &&
		left.RemainInForceItem == right.RemainInForceItem &&
		left.OriginalExpenseSettlement == right.OriginalExpenseSettlement &&
		left.OriginalOtherSettlement == right.OriginalOtherSettlement && left.OtherDeals == right.OtherDeals
}

func terminationInputMatches(snapshot terminationSnapshot, input TerminateContractInput) bool {
	return snapshot.ContractNo == input.ContractNo && snapshot.Reason == input.Reason &&
		snapshot.RemainInForceItem == input.RemainInForceItem &&
		snapshot.OriginalExpenseSettlement == input.OriginalExpenseSettlement &&
		snapshot.OriginalOtherSettlement == input.OriginalOtherSettlement && snapshot.OtherDeals == input.OtherDeals
}

func (s *serviceImpl) loadCompletedSigningFlow(ctx context.Context, contractNo string) (*contractFlowRow, error) {
	var rows []*contractFlowRow
	if err := s.database.GetScan(ctx, &rows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND contract_no = ? AND purpose_code IN ('issue','reissue')
  AND status_code = 'completed' AND gateway_route_no IS NOT NULL
ORDER BY sequence_no DESC, id DESC LIMIT 1`, idccap.DefaultTenantID, contractNo); err != nil {
		return nil, gerror.Wrap(err, "load completed contract flow")
	}
	if len(rows) != 1 || rows[0] == nil ||
		!customerGatewayRoutePattern.MatchString(pointerString(rows[0].GatewayRouteNo)) {
		return nil, gerror.New("completed provider flow is unavailable for termination")
	}
	return rows[0], nil
}

func (s *serviceImpl) reserveTerminationFlow(
	ctx context.Context,
	contract *contractRow,
	originalFlow *contractFlowRow,
	expectedVersion int64,
	idempotencyDigest string,
	requestReference string,
	requestDigest string,
	deadline time.Time,
) (*contractFlowRow, int64, bool, error) {
	var flow *contractFlowRow
	var version int64
	launchRequired := false
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, key := range []string{"contract-flow:" + contract.ContractNo, "contract-flow-idem:" + idempotencyDigest} {
			if _, err := tx.GetValue("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
				return gerror.Wrap(err, "lock contract termination")
			}
		}
		current, err := lockContractRow(ctx, tx, contract.ContractNo)
		if err != nil {
			return err
		}
		version = current.Version
		var existing []*contractFlowRow
		if err = tx.Ctx(ctx).GetScan(&existing, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND contract_no = ? AND (purpose_code = 'termination' OR idempotency_sha256 = ?)
ORDER BY CASE WHEN idempotency_sha256 = ? THEN 0 ELSE 1 END, id ASC
LIMIT 2 FOR UPDATE`, idccap.DefaultTenantID, contract.ContractNo, idempotencyDigest, idempotencyDigest); err != nil {
			return gerror.Wrap(err, "load contract termination flow")
		}
		if len(existing) == 1 && existing[0] != nil && existing[0].IdempotencySHA256 == idempotencyDigest {
			flow = existing[0]
			if flow.RequestSnapshotRef != requestReference || flow.RequestSnapshotSHA256 != requestDigest {
				return gerror.New("contract termination idempotency key conflicts with another request")
			}
			if flow.StatusCode == "signing" || flow.StatusCode == "result_unknown" {
				return nil
			}
			return gerror.New("existing contract termination cannot be submitted again")
		}
		if len(existing) != 0 || current.StatusCode != "completed" || current.Version != expectedVersion {
			return gerror.New("completed contract state changed")
		}
		sequenceValue, err := tx.GetValue(`SELECT COALESCE(MAX(sequence_no), 0) + 1
FROM plugin_idc_compliance_contract_flow WHERE tenant_id = ? AND contract_no = ?`,
			idccap.DefaultTenantID, current.ContractNo)
		if err != nil || sequenceValue.Int() < 1 {
			return gerror.New("contract termination sequence is unavailable")
		}
		nowValue, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if err != nil {
			return err
		}
		now := nowValue.Time().UTC()
		flowNo, err := newFlowNo(s.random)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`
INSERT INTO plugin_idc_compliance_contract_flow
    (tenant_id, flow_no, contract_no, sequence_no, purpose_code, provider_code,
     idempotency_sha256, request_snapshot_ref, request_snapshot_sha256,
     status_code, available_at, expires_at, version, created_at, updated_at)
VALUES (?, ?, ?, ?, 'termination', ?, ?, ?, ?, 'submitting', ?, ?, 1, ?, ?)`,
			idccap.DefaultTenantID, flowNo, current.ContractNo, sequenceValue.Int(),
			ProviderTencentESSBasic, idempotencyDigest, requestReference, requestDigest,
			now, deadline, now, now); err != nil {
			return gerror.Wrap(err, "reserve contract termination flow")
		}
		result, err := tx.Exec(`UPDATE plugin_idc_compliance_contract
SET version = version + 1, updated_at = ?
WHERE tenant_id = ? AND contract_no = ? AND version = ? AND status_code = 'completed'`,
			now, idccap.DefaultTenantID, current.ContractNo, current.Version)
		if err != nil {
			return gerror.Wrap(err, "reserve contract termination version")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract termination state changed")
		}
		version = current.Version + 1
		flow = &contractFlowRow{FlowNo: flowNo, ContractNo: current.ContractNo,
			IdempotencySHA256: idempotencyDigest, RequestSnapshotRef: requestReference,
			RequestSnapshotSHA256: requestDigest, StatusCode: "submitting", ExpiresAt: &deadline, Version: 1}
		if err = s.appendOperationAudit(ctx, "termination", flow, "submitting"); err != nil {
			return err
		}
		launchRequired = true
		return nil
	})
	return flow, version, launchRequired, err
}

func (s *serviceImpl) finishTerminationFlow(
	ctx context.Context,
	reserved *contractFlowRow,
	requestDigest string,
	gatewayRouteNo string,
	gatewayErr error,
	resultUnknown bool,
) (string, int64, error) {
	status := "signing"
	failureCode := ""
	if gatewayErr != nil {
		failureCode = boundedFailureCode(gatewayErr.Error())
		if resultUnknown {
			status = "result_unknown"
		} else {
			status = "failed"
		}
	}
	var version int64
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		flow, err := lockOperationFlow(ctx, tx, reserved.FlowNo)
		if err != nil {
			return err
		}
		if flow.ContractNo != reserved.ContractNo || flow.IdempotencySHA256 != reserved.IdempotencySHA256 ||
			flow.RequestSnapshotSHA256 != requestDigest {
			return gerror.New("submitted contract termination changed unexpectedly")
		}
		if flow.StatusCode == "signing" && gatewayErr == nil {
			value, loadErr := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, flow.ContractNo)
			version = value.Int64()
			return loadErr
		}
		if flow.StatusCode != "submitting" && flow.StatusCode != "result_unknown" {
			return gerror.New("contract termination flow is not mutable")
		}
		nowValue, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if err != nil {
			return err
		}
		now := nowValue.Time().UTC()
		var route any
		if gatewayRouteNo != "" {
			route = gatewayRouteNo
		}
		flowResult, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract_flow
SET gateway_route_no = ?, status_code = ?, submitted_at = CASE WHEN ? IN ('signing','result_unknown') THEN COALESCE(submitted_at, ?) ELSE submitted_at END,
    failure_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND flow_no = ? AND version = ?`, route, status, status, now,
			failureCode, now, idccap.DefaultTenantID, flow.FlowNo, flow.Version)
		if err != nil {
			return gerror.Wrap(err, "finish contract termination flow")
		}
		if affected, affectedErr := flowResult.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract termination flow update conflicted")
		}
		contractResult, err := tx.Exec(`UPDATE plugin_idc_compliance_contract
SET version = version + 1, updated_at = ?
WHERE tenant_id = ? AND contract_no = ? AND status_code = 'completed'`,
			now, idccap.DefaultTenantID, flow.ContractNo)
		if err != nil {
			return gerror.Wrap(err, "finish contract termination version")
		}
		if affected, affectedErr := contractResult.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("contract termination version update conflicted")
		}
		value, err := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, flow.ContractNo)
		if err != nil {
			return err
		}
		version = value.Int64()
		return s.appendOperationAudit(ctx, "termination", flow, status)
	})
	return status, version, err
}

func lockContractRow(ctx context.Context, tx gdb.TX, contractNo string) (*contractRow, error) {
	var rows []*contractRow
	if err := tx.Ctx(ctx).GetScan(&rows, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ? FOR UPDATE`, idccap.DefaultTenantID, contractNo); err != nil {
		return nil, gerror.Wrap(err, "lock electronic contract")
	}
	if len(rows) != 1 || validateStoredRow(rows[0]) != nil {
		return nil, gerror.New("electronic contract is unavailable")
	}
	return rows[0], nil
}

func lockOperationFlow(ctx context.Context, tx gdb.TX, flowNo string) (*contractFlowRow, error) {
	var rows []*contractFlowRow
	if err := tx.Ctx(ctx).GetScan(&rows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND flow_no = ? FOR UPDATE`, idccap.DefaultTenantID, flowNo); err != nil {
		return nil, gerror.Wrap(err, "lock contract operation flow")
	}
	if len(rows) != 1 || rows[0] == nil {
		return nil, gerror.New("contract operation flow is unavailable")
	}
	return rows[0], nil
}

func (s *serviceImpl) appendOperationAudit(
	ctx context.Context,
	purpose string,
	flow *contractFlowRow,
	status string,
) error {
	if flow == nil || (purpose != "reissue" && purpose != "termination") {
		return gerror.New("contract operation audit input is invalid")
	}
	return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.contract." + purpose + "." + status,
		IdempotencyKey:   flow.IdempotencySHA256, OperationCode: "compliance.contract." + purpose,
		TargetType: "contract", TargetNo: flow.ContractNo, ResultCode: status,
		ReasonCode: purpose + "_" + status, RiskCode: "high", CorrelationNo: flow.FlowNo,
		AfterSnapshot: map[string]any{"state": status, "flow_no": flow.FlowNo, "purpose": purpose},
	})
}

func digestJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		clear(payload)
		return "", gerror.New("contract operation request is invalid")
	}
	defer clear(payload)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func validReleaseText(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if strings.TrimSpace(value) != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsPunct(character) ||
			unicode.IsSpace(character)) {
			return false
		}
	}
	return true
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
