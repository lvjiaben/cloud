package contractsigner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type signerRow struct {
	ID                          int64      `orm:"id"`
	SignerNo                    string     `orm:"signer_no"`
	CustomerNo                  string     `orm:"customer_no"`
	ProfileSnapshotRef          string     `orm:"profile_snapshot_ref"`
	ProfileSnapshotSHA256       string     `orm:"profile_snapshot_sha256"`
	MaskedName                  string     `orm:"masked_name"`
	MaskedIdentityNo            string     `orm:"masked_identity_no"`
	MaskedPhone                 string     `orm:"masked_phone"`
	ConsentPolicyVersion        string     `orm:"consent_policy_version"`
	ConsentOccurredAt           time.Time  `orm:"consent_occurred_at"`
	IdentityStatusCode          string     `orm:"identity_status_code"`
	IdentityAttemptNo           *string    `orm:"identity_attempt_no"`
	IdentityVerificationVersion int64      `orm:"identity_verification_version"`
	IdentityVerifiedAt          *time.Time `orm:"identity_verified_at"`
	PhoneStatusCode             string     `orm:"phone_status_code"`
	PhoneVerificationNo         *string    `orm:"phone_verification_no"`
	PhoneVerificationVersion    int64      `orm:"phone_verification_version"`
	PhoneVerifiedAt             *time.Time `orm:"phone_verified_at"`
	StatusCode                  string     `orm:"status_code"`
	AuthorizationVersion        int64      `orm:"authorization_version"`
	CreationDigest              string     `orm:"creation_digest"`
	DisabledAt                  *time.Time `orm:"disabled_at"`
	Version                     int64      `orm:"version"`
	CreatedAt                   time.Time  `orm:"created_at"`
	UpdatedAt                   time.Time  `orm:"updated_at"`
}

func (s *serviceImpl) ListCurrent(ctx context.Context) ([]Signer, error) {
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	var rows []signerRow
	if err = s.database.Model(profileTable).Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).
		Where("customer_no", subject.CustomerNo).OrderDesc("created_at").OrderDesc("id").Limit(100).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	items := make([]Signer, len(rows))
	for index := range rows {
		items[index] = projectSigner(rows[index])
	}
	return items, nil
}

func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*Signer, error) {
	defer clear(input.CertName)
	defer clear(input.CertNo)
	defer clear(input.Phone)
	if input.CertType != "IDENTITY_CARD" || !input.ConsentAccepted || input.ConsentPolicyVersion != ConsentPolicyVersion ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	profile, err := validatePrivateProfile(input.CertName, input.CertNo, input.Phone)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	defer profile.clear()
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	creationDigest := digest(subject.CustomerNo, input.IdempotencyKey)
	if existing, loadErr := s.loadByCreation(ctx, subject.CustomerNo, creationDigest); loadErr != nil {
		return nil, bizerr.WrapCode(loadErr, CodeUnavailable)
	} else if existing != nil {
		item := projectSigner(*existing)
		return &item, nil
	}
	signerNo := "csgn_" + creationDigest[:32]
	challenge, err := randomBytes(32)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(challenge)
	payload := encodeProfile(challenge, profile)
	if len(payload) == 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	defer clear(payload)
	snapshotDigest := profileDigest(payload)
	created, err := s.profileManager.Create(ctx, "contract-signer-profile:"+signerNo, profileProducer(payload))
	if err != nil || created.Handle.IsZero() || !created.Metadata.Configured {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if existing, loadErr := s.loadByCreation(txCtx, subject.CustomerNo, creationDigest); loadErr != nil {
			return loadErr
		} else if existing != nil {
			return nil
		}
		_, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_compliance_contract_signer_profile
(tenant_id,signer_no,customer_no,profile_snapshot_ref,profile_snapshot_sha256,masked_name,masked_identity_no,masked_phone,
 consent_policy_version,consent_occurred_at,identity_status_code,phone_status_code,status_code,authorization_version,creation_digest,version)
VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,'unverified','unverified','active',1,?,1)`, idccap.DefaultTenantID, signerNo,
			subject.CustomerNo, created.Handle.OpaqueRef(), snapshotDigest, maskName(profile.name), maskIdentity(profile.certNo),
			maskPhone(profile.phone), input.ConsentPolicyVersion, creationDigest)
		if insertErr != nil {
			return insertErr
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"signer_no": ""}, IdempotencyScope: "compliance.contract-signer.create",
			IdempotencyKey: input.IdempotencyKey, OperationCode: "compliance.contract-signer.create", TargetType: "contract_signer",
			TargetNo: signerNo, CustomerNo: subject.CustomerNo, ResultCode: "succeeded", ReasonCode: "enterprise_admin_created",
			RiskCode: "high", AfterSnapshot: map[string]any{"identity_status": "unverified", "phone_status": "unverified", "authorization_version": 1}})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	row, err := s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	item := projectSigner(*row)
	return &item, nil
}

func (s *serviceImpl) Disable(ctx context.Context, signerNo string, expectedVersion int64, idempotencyKey string) (*Signer, error) {
	if !validBusinessNo(signerNo) || expectedVersion <= 0 || !validIdempotencyKey(idempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	row, err := s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if row.StatusCode == "disabled" {
		item := projectSigner(*row)
		return &item, nil
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_compliance_contract_signer_profile
SET status_code='disabled',disabled_at=CURRENT_TIMESTAMP,authorization_version=authorization_version+1,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND signer_no=? AND customer_no=? AND status_code='active' AND version=?`, idccap.DefaultTenantID,
			signerNo, subject.CustomerNo, expectedVersion)
		if updateErr != nil {
			return updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if rows != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.contract-signer.disable",
			IdempotencyKey: idempotencyKey, OperationCode: "compliance.contract-signer.disable", TargetType: "contract_signer", TargetNo: signerNo,
			CustomerNo: subject.CustomerNo, ResultCode: "succeeded", ReasonCode: "enterprise_admin_disabled", RiskCode: "high",
			BeforeSnapshot: map[string]any{"version": expectedVersion, "status": "active"}, AfterSnapshot: map[string]any{"version": expectedVersion + 1, "status": "disabled"}})
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	row, err = s.loadOwned(ctx, subject.CustomerNo, signerNo)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	item := projectSigner(*row)
	return &item, nil
}

func (s *serviceImpl) ListCandidates(ctx context.Context, customerNo string) ([]Candidate, error) {
	return s.listCandidates(ctx, customerNo, PermissionContractCreate, customerNo)
}

// ListReissueCandidates returns the same verified directory under the exact contract reissue grant.
func (s *serviceImpl) ListReissueCandidates(
	ctx context.Context,
	customerNo string,
	contractNo string,
) ([]Candidate, error) {
	return s.listCandidates(ctx, customerNo, PermissionContractReissue, contractNo)
}

func (s *serviceImpl) listCandidates(
	ctx context.Context,
	customerNo string,
	permission string,
	targetNo string,
) ([]Candidate, error) {
	if !validBusinessNo(customerNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: permission, TargetNo: targetNo})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if actor <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var rows []signerRow
	if err = s.database.Model(profileTable).Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).Where("customer_no", customerNo).
		Where("identity_status_code", "verified").Where("phone_status_code", "verified").Where("status_code", "active").
		OrderAsc("masked_name").OrderAsc("signer_no").Limit(100).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	items := make([]Candidate, 0, len(rows))
	for _, row := range rows {
		if row.IdentityVerifiedAt == nil || row.PhoneVerifiedAt == nil {
			continue
		}
		items = append(items, Candidate{SignerNo: row.SignerNo, MaskedName: row.MaskedName, MaskedPhone: row.MaskedPhone,
			IdentityVerificationVersion: row.IdentityVerificationVersion, IdentityVerifiedAt: row.IdentityVerifiedAt.UTC(),
			PhoneVerificationVersion: row.PhoneVerificationVersion, PhoneVerifiedAt: row.PhoneVerifiedAt.UTC(),
			AuthorizationVersion: row.AuthorizationVersion, Version: row.Version})
	}
	return items, nil
}

func (s *serviceImpl) WithCandidate(ctx context.Context, customerNo, signerNo string, consume FrozenSignerConsumer) error {
	if !validBusinessNo(customerNo) || !validBusinessNo(signerNo) || consume == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadOwned(ctx, customerNo, signerNo)
	if err != nil || row == nil || row.StatusCode != "active" || row.IdentityStatusCode != "verified" || row.PhoneStatusCode != "verified" ||
		row.IdentityVerifiedAt == nil || row.PhoneVerifiedAt == nil || row.PhoneVerificationNo == nil {
		return bizerr.NewCode(CodeVerificationRequired)
	}
	return s.accessProfile(ctx, *row, func(profile *privateProfile) error {
		frozen := FrozenSigner{SignerNo: row.SignerNo, CustomerNo: row.CustomerNo, Name: append([]byte(nil), profile.name...),
			Phone: append([]byte(nil), profile.phone...), IdentityVerificationVersion: row.IdentityVerificationVersion,
			IdentityVerifiedAt: row.IdentityVerifiedAt.UTC(), PhoneVerificationNo: *row.PhoneVerificationNo,
			PhoneVerificationVersion: row.PhoneVerificationVersion, PhoneVerifiedAt: row.PhoneVerifiedAt.UTC(),
			AuthorizationVersion: row.AuthorizationVersion}
		defer clear(frozen.Name)
		defer clear(frozen.Phone)
		return consume(frozen)
	})
}

func (s *serviceImpl) loadOwned(ctx context.Context, customerNo, signerNo string) (*signerRow, error) {
	var row signerRow
	if err := s.database.Model(profileTable).Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).Where("customer_no", customerNo).
		Where("signer_no", signerNo).Limit(1).Scan(&row); err != nil {
		return nil, err
	}
	if row.SignerNo == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) loadByCreation(ctx context.Context, customerNo, creationDigest string) (*signerRow, error) {
	var row signerRow
	if err := s.database.Model(profileTable).Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).Where("customer_no", customerNo).
		Where("creation_digest", creationDigest).Limit(1).Scan(&row); err != nil {
		return nil, err
	}
	if row.SignerNo == "" {
		return nil, nil
	}
	return &row, nil
}

func projectSigner(row signerRow) Signer {
	item := Signer{SignerNo: row.SignerNo, MaskedName: row.MaskedName, MaskedIdentityNo: row.MaskedIdentityNo, MaskedPhone: row.MaskedPhone,
		IdentityStatus: row.IdentityStatusCode, IdentityVerificationVersion: row.IdentityVerificationVersion,
		PhoneStatus: row.PhoneStatusCode, PhoneVerificationVersion: row.PhoneVerificationVersion, Status: row.StatusCode,
		AuthorizationVersion: row.AuthorizationVersion, Version: row.Version, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
	if row.IdentityAttemptNo != nil {
		item.IdentityAttemptNo = *row.IdentityAttemptNo
	}
	if row.PhoneVerificationNo != nil {
		item.PhoneVerificationNo = *row.PhoneVerificationNo
	}
	if row.IdentityVerifiedAt != nil {
		value := row.IdentityVerifiedAt.UTC()
		item.IdentityVerifiedAt = &value
	}
	if row.PhoneVerifiedAt != nil {
		value := row.PhoneVerifiedAt.UTC()
		item.PhoneVerifiedAt = &value
	}
	return item
}

func digest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hash.Write([]byte(":" + part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

func validBusinessNo(value string) bool {
	if len(value) < 3 || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}
