package electroniccontractpreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const contractProjectionFields = `
contract_no, customer_no, order_no, subject_type_code, signing_order_code,
required_for_fulfillment,
template_no, template_version, variable_snapshot_ref, variable_snapshot_sha256,
signer_snapshot_sha256, snapshot_key_ref, order_snapshot_sha256,
creation_idempotency_digest, creation_fingerprint, currency_code, amount_minor,
status_code, version, signing_expires_at, created_at`

type contractRow struct {
	ContractNo                string    `orm:"contract_no"`
	CustomerNo                string    `orm:"customer_no"`
	OrderNo                   string    `orm:"order_no"`
	SubjectTypeCode           string    `orm:"subject_type_code"`
	SigningOrderCode          string    `orm:"signing_order_code"`
	RequiredForFulfillment    bool      `orm:"required_for_fulfillment"`
	TemplateNo                string    `orm:"template_no"`
	TemplateVersion           int64     `orm:"template_version"`
	VariableSnapshotRef       string    `orm:"variable_snapshot_ref"`
	VariableSnapshotSHA256    string    `orm:"variable_snapshot_sha256"`
	SignerSnapshotSHA256      string    `orm:"signer_snapshot_sha256"`
	SnapshotKeyRef            string    `orm:"snapshot_key_ref"`
	OrderSnapshotSHA256       string    `orm:"order_snapshot_sha256"`
	CreationIdempotencyDigest string    `orm:"creation_idempotency_digest"`
	CreationFingerprint       string    `orm:"creation_fingerprint"`
	CurrencyCode              string    `orm:"currency_code"`
	AmountMinor               int64     `orm:"amount_minor"`
	StatusCode                string    `orm:"status_code"`
	Version                   int64     `orm:"version"`
	SigningExpiresAt          time.Time `orm:"signing_expires_at"`
	CreatedAt                 time.Time `orm:"created_at"`
}

func (s *serviceImpl) PrepareRequired(
	ctx context.Context,
	input PrepareInput,
) (*PreparedContract, error) {
	return s.prepare(ctx, input.OrderNo, "", input.SignerNo, input.IdempotencyKey, true)
}

func (s *serviceImpl) PrepareManualOptional(
	ctx context.Context,
	input ManualOptionalInput,
) (*PreparedContract, error) {
	orderNo := strings.TrimSpace(input.OrderNo)
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(orderNo, 40) {
		return nil, gerror.New("manual electronic contract authorization input is invalid")
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractCreate,
		TargetNo:   orderNo,
	}); err != nil {
		return nil, err
	}
	input.OrderNo = orderNo
	return s.prepare(ctx, input.OrderNo, input.TemplateNo, input.SignerNo, input.IdempotencyKey, false)
}

// ListManualOptionalTemplates returns only enabled templates that match the
// exact order's verified customer type. The independent create permission is
// sufficient; template-management permissions are deliberately not reused.
func (s *serviceImpl) ListManualOptionalTemplates(
	ctx context.Context,
	orderNo string,
) ([]ManualOptionalTemplate, error) {
	options, err := s.GetManualOptionalOptions(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	return options.Templates, nil
}

// GetManualOptionalOptions returns one safe order context and its templates.
func (s *serviceImpl) GetManualOptionalOptions(
	ctx context.Context,
	orderNo string,
) (*ManualOptionalOptions, error) {
	orderNo = strings.TrimSpace(orderNo)
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(orderNo, 40) {
		return nil, gerror.New("manual electronic contract template input is invalid")
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractCreate,
		TargetNo:   orderNo,
	}); err != nil {
		return nil, err
	}
	exists, err := s.templates.OrderContractExists(ctx, orderNo)
	if err != nil {
		return nil, gerror.Wrap(err, "check optional contract existence")
	}
	if exists {
		return &ManualOptionalOptions{OrderNo: orderNo, Templates: []ManualOptionalTemplate{}}, nil
	}
	order, err := s.runtime.Orders.PrepareOptionalOrder(ctx, orderNo)
	if err != nil {
		return nil, gerror.Wrap(err, "prepare optional contract template order")
	}
	if order == nil {
		return nil, gerror.New("optional contract template order is unavailable")
	}
	defer order.Clear()
	selection := contracttemplatecap.Selection{
		Level:        contracttemplatecap.SelectionLevelOrder,
		CustomerType: contracttemplatecap.CustomerType(order.Customer.CustomerType),
	}
	available, err := s.templates.ListAvailable(ctx, selection)
	if err != nil {
		return nil, gerror.Wrap(err, "list optional contract templates")
	}
	result := make([]ManualOptionalTemplate, 0, len(available))
	for _, template := range available {
		if err = contracttemplatecap.ValidateTemplate(template, selection); err != nil {
			return nil, gerror.Wrap(err, "validate optional contract template")
		}
		result = append(result, ManualOptionalTemplate{
			TemplateNo: template.TemplateNo, VersionNo: template.VersionNo,
			Name: template.Name, ScopeCode: template.ScopeCode,
			SigningOrderCode: template.SigningOrderCode,
			LanguageCode:     template.LanguageCode,
		})
	}
	return &ManualOptionalOptions{
		OrderNo: orderNo, CustomerNo: order.Customer.CustomerNo,
		CustomerType: order.Customer.CustomerType, Templates: result,
	}, nil
}

func (s *serviceImpl) prepare(
	ctx context.Context,
	orderNo, templateNo, signerNo, idempotencyKey string,
	requiredForFulfillment bool,
) (*PreparedContract, error) {
	orderNo = strings.TrimSpace(orderNo)
	templateNo = strings.TrimSpace(templateNo)
	signerNo = strings.TrimSpace(signerNo)
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!validBusinessNo(orderNo, 40) || !validIdempotencyKey(idempotencyKey) ||
		(requiredForFulfillment && templateNo != "") ||
		(!requiredForFulfillment && !validBusinessNo(templateNo, 64)) {
		return nil, gerror.New("electronic contract preparation input is invalid")
	}
	idempotencyDigest := digestText("idc/contract-create-idempotency/v1\x00", idempotencyKey)
	mode := "required"
	if !requiredForFulfillment {
		mode = "manual_optional"
	}
	fingerprint := digestText(
		"idc/contract-create-fingerprint/v2\x00",
		mode+"\x00"+orderNo+"\x00"+templateNo+"\x00"+signerNo,
	)
	existing, err := s.loadExisting(ctx, idempotencyDigest, orderNo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err = validateExisting(existing, orderNo, fingerprint); err != nil {
			return nil, err
		}
		return s.projectPersisted(ctx, existing, true)
	}
	if s.privacyGate == nil {
		return nil, gerror.New("electronic contract privacy policy is unavailable")
	}
	if _, err = s.privacyGate.RequireSensitiveCollection(ctx, "electronic_contract", "electronic_contract_evidence"); err != nil {
		return nil, gerror.Wrap(err, "require electronic contract privacy policy")
	}

	state := new(preparationState)
	defer state.clear()
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return s.prepareInTransaction(ctx, tx, state, orderNo, templateNo, signerNo,
			idempotencyDigest, fingerprint, requiredForFulfillment)
	})
	if err != nil {
		if state.prepared != nil {
			state.prepared.Clear()
		}
		return nil, err
	}
	if state.persisted == nil {
		if state.prepared != nil {
			state.prepared.Clear()
		}
		return nil, gerror.New("prepared electronic contract was not persisted")
	}
	if state.prepared == nil {
		return s.projectPersisted(ctx, state.persisted, true)
	}
	return state.prepared, nil
}

type preparationState struct {
	order     *contractcap.OrderSnapshot
	persisted *contractRow
	bundle    *snapshotBundle
	payload   []byte
	prepared  *PreparedContract
}

func (state *preparationState) clear() {
	if state.order != nil {
		state.order.Clear()
	}
	if state.bundle != nil {
		state.bundle.Clear()
	}
	clear(state.payload)
}

func (s *serviceImpl) prepareInTransaction(
	ctx context.Context,
	tx gdb.TX,
	state *preparationState,
	orderNo, templateNo, signerNo, idempotencyDigest, fingerprint string,
	requiredForFulfillment bool,
) error {
	for _, key := range advisoryLockKeys(idempotencyDigest, orderNo) {
		if _, err := tx.GetValue("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
			return gerror.Wrap(err, "lock contract preparation key")
		}
	}
	current, err := loadExistingTX(ctx, tx, idempotencyDigest, orderNo)
	if err != nil {
		return err
	}
	if current != nil {
		if err = validateExisting(current, orderNo, fingerprint); err != nil {
			return err
		}
		state.persisted = current
		return nil
	}
	state.order, err = s.loadPreparationOrder(ctx, orderNo, templateNo, requiredForFulfillment)
	if err != nil {
		return err
	}
	databaseTime, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return gerror.Wrap(err, "load contract creation time")
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
	customerSigner, err := s.resolveCustomerSigner(ctx, state.order, signerNo)
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
	created, err := s.manager.Create(ctx, "contract-snapshot:"+idempotencyDigest, bundleProducer(state.payload))
	if err != nil {
		return gerror.Wrap(err, "seal contract preparation snapshot")
	}
	return s.persistSealedPreparation(ctx, tx, state, created, digests, orderNo, signerNo,
		idempotencyDigest, fingerprint, requiredForFulfillment)
}

func (s *serviceImpl) persistSealedPreparation(
	ctx context.Context,
	tx gdb.TX,
	state *preparationState,
	created secretcap.CreateResult,
	digests snapshotDigests,
	orderNo, signerNo, idempotencyDigest, fingerprint string,
	requiredForFulfillment bool,
) error {
	if created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		return gerror.New("sealed contract preparation snapshot metadata is invalid")
	}
	reference := created.Handle.OpaqueRef()
	if reference == "" {
		return gerror.New("contract snapshot reference is unavailable")
	}
	if created.Reused {
		state.bundle.Clear()
		state.bundle = nil
		return s.withSnapshotBundle(ctx, created.Handle, "compliance.contract-recovery", nil,
			func(recovered *snapshotBundle) error {
				if !recoveredSnapshotMatches(recovered, orderNo, signerNo, requiredForFulfillment) {
					return gerror.New("recovered contract snapshot belongs to another order")
				}
				recoveredDigests, err := digestBundle(recovered)
				if err != nil {
					return err
				}
				if err = s.persistPrepared(ctx, tx, recovered, recoveredDigests, reference,
					idempotencyDigest, fingerprint); err != nil {
					return err
				}
				state.persisted = rowFromBundle(recovered, recoveredDigests, reference, idempotencyDigest, fingerprint)
				state.prepared = projectPrepared(recovered, true)
				return nil
			})
	}
	if state.bundle == nil || state.bundle.ContractNo == "" {
		return gerror.New("contract snapshot bundle is unavailable")
	}
	if err := s.persistPrepared(ctx, tx, state.bundle, digests, reference, idempotencyDigest, fingerprint); err != nil {
		return err
	}
	state.persisted = rowFromBundle(state.bundle, digests, reference, idempotencyDigest, fingerprint)
	state.prepared = projectPrepared(state.bundle, false)
	return nil
}

func recoveredSnapshotMatches(recovered *snapshotBundle, orderNo, signerNo string, requiredForFulfillment bool) bool {
	return recovered != nil && recovered.Order.OrderNo == orderNo &&
		recovered.Order.RequiresContract == requiredForFulfillment &&
		(recovered.CustomerSigner.SignerNo == signerNo || signerNo == "" &&
			recovered.Order.Customer.CustomerType == contractcap.CustomerTypeIndividual)
}

func (s *serviceImpl) loadPreparationOrder(
	ctx context.Context,
	orderNo, templateNo string,
	requiredForFulfillment bool,
) (*contractcap.OrderSnapshot, error) {
	var order *contractcap.OrderSnapshot
	var err error
	if requiredForFulfillment {
		order, err = s.runtime.Orders.PrepareOrder(ctx, orderNo)
	} else {
		order, err = s.runtime.Orders.PrepareOptionalOrder(ctx, orderNo)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "prepare authoritative contract order")
	}
	if order == nil {
		return nil, gerror.New("authoritative contract order is unavailable")
	}
	if !requiredForFulfillment {
		if err = s.applyOptionalTemplate(ctx, order, templateNo); err != nil {
			order.Clear()
			return nil, err
		}
	}
	return order, nil
}

func (s *serviceImpl) applyOptionalTemplate(
	ctx context.Context,
	order *contractcap.OrderSnapshot,
	templateNo string,
) error {
	if s == nil || order == nil || order.RequiresContract {
		return gerror.New("manual optional contract order is invalid")
	}
	return s.applySelectedTemplate(ctx, order, templateNo)
}

func (s *serviceImpl) applySelectedTemplate(
	ctx context.Context,
	order *contractcap.OrderSnapshot,
	templateNo string,
) error {
	if s == nil || order == nil || !validBusinessNo(templateNo, 64) {
		return gerror.New("selected contract template input is invalid")
	}
	selection := contracttemplatecap.Selection{
		Level:        contracttemplatecap.SelectionLevelOrder,
		CustomerType: contracttemplatecap.CustomerType(order.Customer.CustomerType),
	}
	templates, err := s.templates.ResolveAvailable(ctx, selection, []string{templateNo})
	if err != nil {
		return gerror.Wrap(err, "resolve selected contract template")
	}
	if len(templates) != 1 || templates[0].TemplateNo != templateNo {
		return gerror.New("selected contract template is unavailable")
	}
	template := templates[0]
	if err = contracttemplatecap.ValidateTemplate(template, selection); err != nil {
		return gerror.Wrap(err, "validate selected contract template")
	}
	order.Template = contractcap.TemplateSnapshot{
		TemplateNo: template.TemplateNo, VersionNo: template.VersionNo,
		Name: template.Name, LanguageCode: template.LanguageCode,
		ScopeCode: template.ScopeCode, SigningOrderCode: template.SigningOrderCode,
	}
	for index := range order.Items {
		name := order.Items[index].ProductNameZhCN
		if template.LanguageCode == "en-US" {
			name = order.Items[index].ProductNameEnUS
		}
		if name == "" || name != strings.TrimSpace(name) {
			return gerror.New("contract product name is unavailable in template language")
		}
		order.Items[index].ContractProductName = name
	}
	return nil
}

func (s *serviceImpl) persistPrepared(
	ctx context.Context,
	tx gdb.TX,
	bundle *snapshotBundle,
	digests snapshotDigests,
	reference, idempotencyDigest, fingerprint string,
) error {
	if s == nil || tx == nil || bundle == nil || reference == "" {
		return gerror.New("prepared electronic contract persistence input is invalid")
	}
	if _, insertErr := tx.Exec(`
INSERT INTO plugin_idc_compliance_contract
    (tenant_id, contract_no, customer_no, principal_no, order_no,
     contract_type_code, required_for_fulfillment, template_no, template_version,
     variable_snapshot_ref, variable_snapshot_sha256, signer_snapshot_sha256,
     currency_code, amount_minor, status_code, version, subject_type_code,
     signing_order_code, snapshot_key_ref, order_snapshot_sha256,
     creation_idempotency_digest, creation_fingerprint, signing_expires_at,
     created_at, updated_at)
VALUES
    (?, ?, ?, ?, ?, 'service_purchase', ?, ?, ?, ?, ?, ?, ?, ?, 'pending_launch', 1,
     ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idccap.DefaultTenantID, bundle.ContractNo, bundle.Order.Customer.CustomerNo,
		"customer:"+bundle.Order.Customer.CustomerNo, bundle.Order.OrderNo,
		bundle.Order.RequiresContract,
		bundle.Order.Template.TemplateNo, bundle.Order.Template.VersionNo,
		reference, digests.Variables, digests.Signers,
		bundle.Order.CurrencyCode, bundle.Order.AmountMinor,
		string(bundle.Order.Customer.CustomerType), bundle.Order.Template.SigningOrderCode,
		reference, digests.Order, idempotencyDigest, fingerprint,
		bundle.SigningExpiresAt, bundle.CreatedAt, bundle.CreatedAt,
	); insertErr != nil {
		return gerror.Wrap(insertErr, "insert prepared electronic contract")
	}
	reasonCode := "manual_optional_contract"
	if bundle.Order.RequiresContract {
		reasonCode = "required_contract_order"
	}
	if auditErr := s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"state": "not_created"},
		IdempotencyScope: "compliance.contract.prepare",
		IdempotencyKey:   idempotencyDigest, OperationCode: "compliance.contract.prepare",
		TargetType: "contract", TargetNo: bundle.ContractNo,
		CustomerNo: bundle.Order.Customer.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded,
		ReasonCode: reasonCode, RiskCode: "high",
		CorrelationNo: bundle.Order.OrderNo,
		AfterSnapshot: map[string]any{
			"state": "pending_launch", "order_no": bundle.Order.OrderNo,
			"template_no":              bundle.Order.Template.TemplateNo,
			"template_version":         bundle.Order.Template.VersionNo,
			"required_for_fulfillment": bundle.Order.RequiresContract,
			"signing_expires_at":       bundle.SigningExpiresAt.UnixMilli(),
		},
	}); auditErr != nil {
		return gerror.Wrap(auditErr, "append contract preparation audit")
	}
	return nil
}

func rowFromBundle(
	bundle *snapshotBundle,
	digests snapshotDigests,
	reference, idempotencyDigest, fingerprint string,
) *contractRow {
	return &contractRow{
		ContractNo: bundle.ContractNo, CustomerNo: bundle.Order.Customer.CustomerNo,
		OrderNo: bundle.Order.OrderNo, SubjectTypeCode: string(bundle.Order.Customer.CustomerType),
		SigningOrderCode:       bundle.Order.Template.SigningOrderCode,
		RequiredForFulfillment: bundle.Order.RequiresContract,
		TemplateNo:             bundle.Order.Template.TemplateNo, TemplateVersion: bundle.Order.Template.VersionNo,
		VariableSnapshotRef: reference, VariableSnapshotSHA256: digests.Variables,
		SignerSnapshotSHA256: digests.Signers, SnapshotKeyRef: reference,
		OrderSnapshotSHA256: digests.Order, CreationIdempotencyDigest: idempotencyDigest,
		CreationFingerprint: fingerprint, CurrencyCode: bundle.Order.CurrencyCode,
		AmountMinor: bundle.Order.AmountMinor, StatusCode: "pending_launch",
		Version:          1,
		SigningExpiresAt: bundle.SigningExpiresAt, CreatedAt: bundle.CreatedAt,
	}
}

func (s *serviceImpl) loadExisting(
	ctx context.Context,
	idempotencyDigest, orderNo string,
) (*contractRow, error) {
	var rows []*contractRow
	if err := s.database.GetScan(ctx, &rows, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND (creation_idempotency_digest = ? OR order_no = ?)
ORDER BY CASE WHEN creation_idempotency_digest = ? THEN 0 ELSE 1 END, id ASC
LIMIT 2`, idccap.DefaultTenantID, idempotencyDigest, orderNo, idempotencyDigest); err != nil {
		return nil, gerror.Wrap(err, "load existing contract preparation")
	}
	return oneExisting(rows)
}

func loadExistingTX(
	ctx context.Context,
	tx gdb.TX,
	idempotencyDigest, orderNo string,
) (*contractRow, error) {
	var rows []*contractRow
	if err := tx.GetScan(&rows, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND (creation_idempotency_digest = ? OR order_no = ?)
ORDER BY CASE WHEN creation_idempotency_digest = ? THEN 0 ELSE 1 END, id ASC
LIMIT 2`, idccap.DefaultTenantID, idempotencyDigest, orderNo, idempotencyDigest); err != nil {
		return nil, gerror.Wrap(err, "lock existing contract preparation")
	}
	return oneExisting(rows)
}

func oneExisting(rows []*contractRow) (*contractRow, error) {
	if len(rows) > 1 {
		return nil, gerror.New("contract idempotency key is already bound to another order")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if rows[0] == nil {
		return nil, gerror.New("existing contract preparation is unavailable")
	}
	return rows[0], nil
}

func validateExisting(row *contractRow, orderNo, fingerprint string) error {
	if row == nil || row.OrderNo != orderNo || row.CreationFingerprint != fingerprint ||
		row.ContractNo == "" || row.SnapshotKeyRef == "" ||
		row.VariableSnapshotRef != row.SnapshotKeyRef ||
		!validHexDigest(row.VariableSnapshotSHA256) ||
		!validHexDigest(row.SignerSnapshotSHA256) ||
		!validHexDigest(row.OrderSnapshotSHA256) ||
		row.SigningExpiresAt.IsZero() ||
		!row.SigningExpiresAt.Equal(row.CreatedAt.Add(SigningValidity)) {
		return gerror.New("existing contract preparation conflicts with this request")
	}
	return nil
}

func validHexDigest(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	clear(decoded)
	return err == nil
}

func advisoryLockKeys(idempotencyDigest, orderNo string) []string {
	orderLockKey, _ := contracttemplatecap.OrderContractLockKey(orderNo)
	keys := []string{
		"contract-create-idempotency:" + idempotencyDigest,
		orderLockKey,
	}
	sort.Strings(keys)
	return keys
}

func newContractNo(random io.Reader) (string, error) {
	if random == nil {
		return "", gerror.New("contract number random source is unavailable")
	}
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		clear(value)
		return "", gerror.Wrap(err, "generate contract number")
	}
	defer clear(value)
	return "CTR_" + strings.ToUpper(hex.EncodeToString(value)), nil
}

func digestText(scope, value string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(scope))
	_, _ = hash.Write([]byte(value))
	return hex.EncodeToString(hash.Sum(nil))
}

func validBusinessNo(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || value != strings.TrimSpace(value) ||
		!utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
