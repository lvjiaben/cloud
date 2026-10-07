// This file implements customer-profile reads and replay-safe self-service mutations.

package customer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

const (
	idempotencyStatusInProgress = "in_progress"
	idempotencyStatusSucceeded  = "succeeded"
	idempotencyResourceCustomer = "customer"
	idempotencyRetention        = 30 * 24 * time.Hour
)

type daoPersistence struct {
	audit  transitionAuditWriter
	tasks  asynctask.Service
	now    func() time.Time
	random io.Reader
}

type transitionAuditWriter interface {
	AppendTransition(ctx context.Context, input audit.TransitionInput) (*audit.Event, error)
}

type updateResponseSnapshot struct {
	Version int64 `json:"version"`
}

type profileAuditSnapshot struct {
	Version            int64  `json:"version"`
	CountryCode        string `json:"country_code"`
	PreferredLocale    string `json:"preferred_locale"`
	DisplayNameChanged bool   `json:"display_name_changed"`
}

func newDAOPersistence(auditWriter transitionAuditWriter, taskSvc asynctask.Service) *daoPersistence {
	return &daoPersistence{audit: auditWriter, tasks: taskSvc, now: time.Now, random: rand.Reader}
}

func (p *daoPersistence) load(ctx context.Context, customerNo string) (*Profile, error) {
	return p.loadProfile(ctx, customerNo, false)
}

func (p *daoPersistence) updateOwnBasics(
	ctx context.Context,
	actorUserID int,
	input normalizedUpdate,
) (*Profile, error) {
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	fingerprint, err := requestFingerprint(input)
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint customer profile update")
	}
	callerScopeDigest := digest(
		"customer-profile-caller-v1",
		input.customerNo+"\x00"+strconv.Itoa(actorUserID),
	)
	idempotencyKeyDigest := digest("customer-profile-idempotency-v1", input.idempotencyKey)

	var result *Profile
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(p.now())
		reservation, reserveErr := reserveProfileUpdate(
			ctx,
			callerScopeDigest,
			idempotencyKeyDigest,
			fingerprint,
			now,
		)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replay {
			var replayErr error
			result, replayErr = p.loadProfileUpdateReplay(ctx, reservation.record, input.customerNo)
			return replayErr
		}
		var updateErr error
		result, updateErr = p.applyOwnBasicsUpdate(ctx, actorUserID, input, now)
		if updateErr != nil {
			return updateErr
		}
		return completeProfileUpdate(ctx, reservation.record, fingerprint, input.customerNo, result.Subject.Version, now)
	})
	return result, err
}

type profileUpdateReservation struct {
	record *entity.IdempotencyRecord
	replay bool
}

func reserveProfileUpdate(
	ctx context.Context,
	callerScopeDigest string,
	idempotencyKeyDigest string,
	fingerprint string,
	now time.Time,
) (profileUpdateReservation, error) {
	expiresAt := now.Add(idempotencyRetention)
	insertResult, err := dao.IdempotencyRecord.Ctx(ctx).Data(do.IdempotencyRecord{
		TenantId:             idccap.DefaultTenantID,
		Scope:                idempotencyScopeUpdateOwn,
		CallerScopeDigest:    callerScopeDigest,
		IdempotencyKeyDigest: idempotencyKeyDigest,
		RequestFingerprint:   fingerprint,
		StatusCode:           idempotencyStatusInProgress,
		ExpiresAt:            &expiresAt,
	}).InsertIgnore()
	if err != nil {
		return profileUpdateReservation{}, gerror.Wrap(err, "reserve customer profile idempotency key")
	}
	inserted, err := insertResult.RowsAffected()
	if err != nil {
		return profileUpdateReservation{}, gerror.Wrap(err, "read customer profile reservation result")
	}
	record, err := loadIdempotencyRecord(ctx, callerScopeDigest, idempotencyKeyDigest, true)
	if err != nil {
		return profileUpdateReservation{}, err
	}
	if record == nil {
		return profileUpdateReservation{}, gerror.New("customer profile idempotency reservation is not readable")
	}
	if record.RequestFingerprint != fingerprint {
		return profileUpdateReservation{}, bizerr.NewCode(CodeIdempotencyConflict)
	}
	if inserted == 0 {
		return profileUpdateReservation{record: record, replay: true}, nil
	}
	if record.StatusCode != idempotencyStatusInProgress || record.ResourceType != "" ||
		record.ResourceNo != "" || record.CompletedAt != nil {
		return profileUpdateReservation{}, gerror.New("new customer profile idempotency record is inconsistent")
	}
	return profileUpdateReservation{record: record}, nil
}

func (p *daoPersistence) loadProfileUpdateReplay(
	ctx context.Context,
	record *entity.IdempotencyRecord,
	customerNo string,
) (*Profile, error) {
	if record.StatusCode != idempotencyStatusSucceeded ||
		record.ResourceType != idempotencyResourceCustomer ||
		record.ResourceNo != customerNo ||
		record.CompletedAt == nil || record.ResponseStatusCode != 200 {
		return nil, gerror.New("customer profile idempotency record is incomplete")
	}
	var replay updateResponseSnapshot
	if err := json.Unmarshal([]byte(record.ResponseSnapshot), &replay); err != nil || replay.Version <= 0 {
		return nil, gerror.New("customer profile idempotency response is invalid")
	}
	profile, err := p.loadProfile(ctx, customerNo, true)
	if err != nil {
		return nil, err
	}
	if profile == nil || profile.Subject.Version < replay.Version {
		return nil, gerror.New("customer profile idempotency result is inconsistent")
	}
	return profile, nil
}

func (p *daoPersistence) applyOwnBasicsUpdate(
	ctx context.Context,
	actorUserID int,
	input normalizedUpdate,
	now time.Time,
) (*Profile, error) {
	before, err := p.loadProfile(ctx, input.customerNo, true)
	if err != nil {
		return nil, err
	}
	if before == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if !statusAllowsOwnMutation(before.Subject.Status) {
		return nil, bizerr.NewCode(CodeStateDenied)
	}
	if before.Subject.Version != input.expectedVersion {
		return nil, bizerr.NewCode(CodeVersionConflict)
	}
	if sameOwnBasics(before, input) {
		return nil, bizerr.NewCode(CodeProfileInvalid)
	}

	after, err := updateOwnBasicsRow(ctx, before, actorUserID, input, now)
	if err != nil {
		return nil, err
	}
	if err = p.appendOwnBasicsAudit(ctx, before, after, input); err != nil {
		return nil, err
	}
	return after, nil
}

func sameOwnBasics(profile *Profile, input normalizedUpdate) bool {
	return profile.Subject.DisplayName == input.displayName &&
		profile.Subject.CountryCode == input.countryCode &&
		profile.Subject.PreferredLocale == input.preferredLocale
}

func updateOwnBasicsRow(
	ctx context.Context,
	before *Profile,
	actorUserID int,
	input normalizedUpdate,
	now time.Time,
) (*Profile, error) {
	nextVersion := before.Subject.Version + 1
	result, err := dao.CustomerSubject.Ctx(ctx).
		Where(do.CustomerSubject{
			TenantId:   idccap.DefaultTenantID,
			CustomerNo: input.customerNo,
			Version:    before.Subject.Version,
		}).
		Data(do.CustomerSubject{
			DisplayName:     input.displayName,
			SearchName:      input.searchName,
			CountryCode:     input.countryCode,
			PreferredLocale: input.preferredLocale,
			Version:         nextVersion,
			UpdatedBy:       actorUserID,
			UpdatedAt:       &now,
		}).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "update owned customer profile")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, gerror.Wrap(err, "read customer profile update result")
	}
	if affected != 1 {
		return nil, bizerr.NewCode(CodeVersionConflict)
	}
	after := cloneProfile(before)
	after.Subject.DisplayName = input.displayName
	after.Subject.CountryCode = input.countryCode
	after.Subject.PreferredLocale = input.preferredLocale
	after.Subject.Version = nextVersion
	after.Subject.UpdatedAt = now
	return after, nil
}

func (p *daoPersistence) appendOwnBasicsAudit(
	ctx context.Context,
	before *Profile,
	after *Profile,
	input normalizedUpdate,
) error {
	_, err := p.audit.AppendTransition(ctx, audit.TransitionInput{
		Fields: audit.Fields{
			IdempotencyScope: idempotencyScopeUpdateOwn,
			IdempotencyKey:   input.idempotencyKey,
			OperationCode:    idempotencyScopeUpdateOwn,
			TargetType:       idempotencyResourceCustomer,
			TargetNo:         input.customerNo,
			CustomerNo:       input.customerNo,
			Result:           audit.ResultSucceeded,
			Risk:             audit.RiskLow,
			ContextSnapshot:  map[string]any{"source": "customer_self_service"},
		},
		BeforeSnapshot: profileAuditSnapshot{
			Version:            before.Subject.Version,
			CountryCode:        before.Subject.CountryCode,
			PreferredLocale:    before.Subject.PreferredLocale,
			DisplayNameChanged: false,
		},
		AfterSnapshot: profileAuditSnapshot{
			Version:            after.Subject.Version,
			CountryCode:        after.Subject.CountryCode,
			PreferredLocale:    after.Subject.PreferredLocale,
			DisplayNameChanged: before.Subject.DisplayName != after.Subject.DisplayName,
		},
	})
	if err != nil {
		return gerror.Wrap(err, "append customer profile audit transition")
	}
	return nil
}

func completeProfileUpdate(
	ctx context.Context,
	record *entity.IdempotencyRecord,
	fingerprint string,
	customerNo string,
	version int64,
	now time.Time,
) error {
	responseJSON, err := json.Marshal(updateResponseSnapshot{Version: version})
	if err != nil {
		return gerror.Wrap(err, "encode customer profile idempotency response")
	}
	completedAt := now
	result, err := dao.IdempotencyRecord.Ctx(ctx).
		Where(do.IdempotencyRecord{
			Id:                 record.Id,
			TenantId:           idccap.DefaultTenantID,
			StatusCode:         idempotencyStatusInProgress,
			RequestFingerprint: fingerprint,
		}).
		Data(do.IdempotencyRecord{
			StatusCode:         idempotencyStatusSucceeded,
			ResourceType:       idempotencyResourceCustomer,
			ResourceNo:         customerNo,
			ResponseStatusCode: 200,
			ResponseSnapshot:   string(responseJSON),
			CompletedAt:        &completedAt,
			UpdatedAt:          &now,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "complete customer profile idempotency record")
	}
	completed, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read customer profile idempotency completion")
	}
	if completed != 1 {
		return gerror.New("customer profile idempotency completion lost its reservation")
	}
	return nil
}

func loadIdempotencyRecord(
	ctx context.Context,
	callerScopeDigest string,
	idempotencyKeyDigest string,
	lock bool,
) (*entity.IdempotencyRecord, error) {
	var row *entity.IdempotencyRecord
	model := dao.IdempotencyRecord.Ctx(ctx).Where(do.IdempotencyRecord{
		TenantId:             idccap.DefaultTenantID,
		Scope:                idempotencyScopeUpdateOwn,
		CallerScopeDigest:    callerScopeDigest,
		IdempotencyKeyDigest: idempotencyKeyDigest,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load customer profile idempotency record")
	}
	return row, nil
}

func (p *daoPersistence) loadProfile(
	ctx context.Context,
	customerNo string,
	lock bool,
) (*Profile, error) {
	var subjectRow *entity.CustomerSubject
	model := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId:   idccap.DefaultTenantID,
		CustomerNo: customerNo,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&subjectRow); err != nil {
		return nil, gerror.Wrap(err, "load owned customer subject")
	}
	if subjectRow == nil {
		return nil, nil
	}
	profile, err := projectSubject(subjectRow)
	if err != nil {
		return nil, err
	}
	var individualRow *entity.CustomerIndividualProfile
	individualModel := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	})
	if lock {
		individualModel = individualModel.LockUpdate()
	}
	if err = individualModel.Scan(&individualRow); err != nil {
		return nil, gerror.Wrap(err, "load individual customer profile")
	}
	if individualRow != nil {
		profile.Individual, err = projectIndividual(individualRow, customerNo)
		if err != nil {
			return nil, err
		}
	}

	var enterpriseRow *entity.CustomerEnterpriseProfile
	enterpriseModel := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	})
	if lock {
		enterpriseModel = enterpriseModel.LockUpdate()
	}
	if err = enterpriseModel.Scan(&enterpriseRow); err != nil {
		return nil, gerror.Wrap(err, "load enterprise customer profile")
	}
	if enterpriseRow != nil {
		profile.Enterprise, err = projectEnterprise(enterpriseRow, customerNo)
		if err != nil {
			return nil, err
		}
	}

	switch profile.Subject.Type {
	case TypeIndividual:
		if profile.Individual == nil {
			err = gerror.New("individual customer profile is unavailable")
		}
	case TypeEnterprise:
		if profile.Enterprise == nil {
			err = gerror.New("enterprise customer profile is unavailable")
		}
	default:
		err = gerror.New("customer subject has unsupported type")
	}
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func projectSubject(row *entity.CustomerSubject) (*Profile, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || !validCustomerNo(row.CustomerNo) ||
		row.CustomerCode < 10000 ||
		!validType(Type(row.CustomerTypeCode)) || !validStatus(Status(row.StatusCode)) ||
		row.Version <= 0 || row.CreatedAt == nil || row.UpdatedAt == nil ||
		!validCountryCode(row.CountryCode) || !validLocale(row.PreferredLocale) {
		return nil, gerror.New("customer subject projection is inconsistent")
	}
	if Status(row.StatusCode) == StatusAnonymized {
		if row.AnonymizedAt == nil {
			return nil, gerror.New("anonymized customer has no completion time")
		}
	} else if !validDisplayName(row.DisplayName) {
		return nil, gerror.New("customer subject display name is inconsistent")
	}
	return &Profile{Subject: Subject{
		CustomerNo:       row.CustomerNo,
		CustomerCode:     row.CustomerCode,
		Type:             Type(row.CustomerTypeCode),
		Status:           Status(row.StatusCode),
		StatusReasonCode: row.StatusReasonCode,
		StatusReason:     row.StatusReason,
		DisplayName:      row.DisplayName,
		CountryCode:      row.CountryCode,
		PreferredLocale:  row.PreferredLocale,
		Version:          row.Version,
		AnonymizedAt:     copyTime(row.AnonymizedAt),
		CreatedAt:        databaseTime(*row.CreatedAt),
		UpdatedAt:        databaseTime(*row.UpdatedAt),
	}}, nil
}

func projectIndividual(
	row *entity.CustomerIndividualProfile,
	customerNo string,
) (*IndividualProfile, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || row.CustomerNo != customerNo ||
		Type(row.CustomerTypeCode) != TypeIndividual ||
		!validVerificationStatus(VerificationStatus(row.IdentityStatusCode)) ||
		row.Version <= 0 || row.UpdatedAt == nil {
		return nil, gerror.New("individual customer profile projection is inconsistent")
	}
	return &IndividualProfile{
		LegalName:                row.LegalName,
		LegalNameMasked:          row.LegalNameMasked,
		IdentityIdentifierMasked: row.IdentityIdentifierMasked,
		IdentityCaseNo:           row.IdentityCaseNo,
		IdentityStatus:           VerificationStatus(row.IdentityStatusCode),
		IdentityVerifiedAt:       copyTime(row.IdentityVerifiedAt),
		IdentityExpiresAt:        copyTime(row.IdentityExpiresAt),
		Version:                  row.Version,
		UpdatedAt:                databaseTime(*row.UpdatedAt),
	}, nil
}

func projectEnterprise(
	row *entity.CustomerEnterpriseProfile,
	customerNo string,
) (*EnterpriseProfile, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || row.CustomerNo != customerNo ||
		Type(row.CustomerTypeCode) != TypeEnterprise ||
		!validCountryCode(row.RegistrationCountryCode) ||
		!validVerificationStatus(VerificationStatus(row.VerificationStatusCode)) ||
		row.Version <= 0 || row.UpdatedAt == nil {
		return nil, gerror.New("enterprise customer profile projection is inconsistent")
	}
	return &EnterpriseProfile{
		LegalName:                    row.LegalName,
		RegistrationCountryCode:      row.RegistrationCountryCode,
		RegistrationIdentifierMasked: row.RegistrationIdentifierMasked,
		IndustryCode:                 row.IndustryCode,
		VerificationCaseNo:           row.VerificationCaseNo,
		VerificationStatus:           VerificationStatus(row.VerificationStatusCode),
		VerifiedAt:                   copyTime(row.VerifiedAt),
		ExpiresAt:                    copyTime(row.ExpiresAt),
		Version:                      row.Version,
		UpdatedAt:                    databaseTime(*row.UpdatedAt),
	}, nil
}

func statusAllowsOwnMutation(status Status) bool {
	return status == StatusPendingActivation || status == StatusActive || status == StatusRestricted
}

func cloneProfile(source *Profile) *Profile {
	clone := *source
	clone.Subject.AnonymizedAt = copyTime(source.Subject.AnonymizedAt)
	if source.Individual != nil {
		individual := *source.Individual
		individual.IdentityVerifiedAt = copyTime(source.Individual.IdentityVerifiedAt)
		individual.IdentityExpiresAt = copyTime(source.Individual.IdentityExpiresAt)
		clone.Individual = &individual
	}
	if source.Enterprise != nil {
		enterprise := *source.Enterprise
		enterprise.VerifiedAt = copyTime(source.Enterprise.VerifiedAt)
		enterprise.ExpiresAt = copyTime(source.Enterprise.ExpiresAt)
		clone.Enterprise = &enterprise
	}
	return &clone
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := databaseTime(*value)
	return &copyValue
}

func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}
