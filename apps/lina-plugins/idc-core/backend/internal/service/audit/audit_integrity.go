// This file defines strict persisted envelopes and verifies append-only row integrity.

package audit

import (
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	integritySchemaVersion = 1
	rowIntegrityDomain     = "audit-persisted-row-v1"
	rawReplayDomain        = "audit-raw-replay-v1"
)

type securitySnapshot struct {
	EntryKind              string `json:"entry_kind"`
	ActorAudienceCode      string `json:"actor_audience_code"`
	ActorReferenceHMAC     string `json:"actor_reference_hmac"`
	DataScope              int    `json:"data_scope"`
	Delegated              bool   `json:"delegated"`
	RequiredPermission     string `json:"required_permission"`
	RequiredGranted        bool   `json:"required_granted"`
	WildcardGranted        bool   `json:"wildcard_granted"`
	PlatformAdmin          bool   `json:"platform_admin"`
	PermissionSnapshotHMAC string `json:"permission_snapshot_hmac"`
	PlatformBypass         bool   `json:"platform_bypass"`
}

type integritySnapshot struct {
	Version       int    `json:"version"`
	KeyID         string `json:"key_id"`
	ReplayKeyID   string `json:"replay_key_id"`
	RawReplayHMAC string `json:"raw_replay_hmac"`
	RowHMAC       string `json:"row_hmac"`
}

type integrityCore struct {
	Version       int    `json:"version"`
	KeyID         string `json:"key_id"`
	ReplayKeyID   string `json:"replay_key_id"`
	RawReplayHMAC string `json:"raw_replay_hmac"`
}

type stateEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Data          map[string]any `json:"data"`
}

type contextEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	Data          map[string]any    `json:"data"`
	Security      securitySnapshot  `json:"security"`
	Integrity     integritySnapshot `json:"integrity"`
}

type contextCoreEnvelope struct {
	SchemaVersion int              `json:"schema_version"`
	Data          map[string]any   `json:"data"`
	Security      securitySnapshot `json:"security"`
	Integrity     integrityCore    `json:"integrity"`
}

type storedStateEnvelope struct {
	original json.RawMessage
	redacted json.RawMessage
}

type storedContextEnvelope struct {
	originalCore json.RawMessage
	redacted     json.RawMessage
	data         map[string]any
	security     securitySnapshot
	integrity    integritySnapshot
}

type verifiedEvidence struct {
	before  json.RawMessage
	after   json.RawMessage
	context json.RawMessage
	parsed  storedContextEnvelope
}

type rowIntegrityPayload struct {
	Version                int             `json:"version"`
	InternalID             int64           `json:"internal_id"`
	TenantID               int             `json:"tenant_id"`
	AuditNo                string          `json:"audit_no"`
	OccurredAt             string          `json:"occurred_at"`
	ActorType              string          `json:"actor_type"`
	ActorUserID            int             `json:"actor_user_id"`
	ActingUserID           int             `json:"acting_user_id"`
	ActorAudienceCode      string          `json:"actor_audience_code"`
	ActorCustomerNo        string          `json:"actor_customer_no"`
	CustomerNo             string          `json:"customer_no"`
	OperationCode          string          `json:"operation_code"`
	TargetType             string          `json:"target_type"`
	TargetNo               string          `json:"target_no"`
	ResultCode             string          `json:"result_code"`
	ReasonCode             string          `json:"reason_code"`
	RiskCode               string          `json:"risk_code"`
	BeforeSnapshot         json.RawMessage `json:"before_snapshot"`
	AfterSnapshot          json.RawMessage `json:"after_snapshot"`
	ContextCore            json.RawMessage `json:"context_core"`
	RequestID              string          `json:"request_id"`
	TraceID                string          `json:"trace_id"`
	CorrelationNo          string          `json:"correlation_no"`
	IPDigest               string          `json:"ip_digest"`
	UserAgentDigest        string          `json:"user_agent_digest"`
	NumberType             string          `json:"number_type"`
	ReservationScopeDigest string          `json:"reservation_scope_digest"`
	IdempotencyKeyDigest   string          `json:"idempotency_key_digest"`
	OwnerType              string          `json:"owner_type"`
	OwnerInternalID        int64           `json:"owner_internal_id"`
	OwnerBusinessNo        string          `json:"owner_business_no"`
	OwnerAssignedAt        string          `json:"owner_assigned_at"`
}

// encodeContextCore creates the exact context material covered by the row MAC;
// RowHMAC is deliberately absent to avoid a circular integrity input.
func encodeContextCore(
	data map[string]any,
	security securitySnapshot,
	integrity integritySnapshot,
) (json.RawMessage, error) {
	encoded, err := json.Marshal(contextCoreEnvelope{
		SchemaVersion: snapshotSchemaVersion,
		Data:          data,
		Security:      security,
		Integrity: integrityCore{
			Version:       integrity.Version,
			KeyID:         integrity.KeyID,
			ReplayKeyID:   integrity.ReplayKeyID,
			RawReplayHMAC: integrity.RawReplayHMAC,
		},
	})
	if err != nil {
		return nil, gerror.Wrap(err, "encode audit context integrity core")
	}
	return encoded, nil
}

// encodeContextSnapshot serializes the strict final envelope after the row MAC
// has been computed over its non-circular context core.
func (s *serviceImpl) encodeContextSnapshot(
	data map[string]any,
	security securitySnapshot,
	integrity integritySnapshot,
) (json.RawMessage, error) {
	encoded, err := json.Marshal(contextEnvelope{
		SchemaVersion: snapshotSchemaVersion,
		Data:          data,
		Security:      security,
		Integrity:     integrity,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "encode audit context snapshot")
	}
	if len(encoded) > s.config.MaximumSnapshotBytes {
		return nil, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	return encoded, nil
}

// rowIntegrityHMAC covers every business evidence scalar, all three snapshot
// envelopes, the business-number reservation, and its stable owner semantics.
func rowIntegrityHMAC(
	key []byte,
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
	contextCore json.RawMessage,
) (string, error) {
	if row == nil || row.Id <= 0 || row.OccurredAt == nil || reservation == nil ||
		reservation.OwnerInternalId <= 0 || reservation.AssignedAt == nil {
		return "", gerror.New("audit integrity input is incomplete")
	}
	payload := rowIntegrityPayload{
		Version:                integritySchemaVersion,
		InternalID:             row.Id,
		TenantID:               row.TenantId,
		AuditNo:                row.AuditNo,
		OccurredAt:             canonicalIntegrityTime(*row.OccurredAt),
		ActorType:              row.ActorType,
		ActorUserID:            row.ActorUserId,
		ActingUserID:           row.ActingUserId,
		ActorAudienceCode:      row.ActorAudienceCode,
		ActorCustomerNo:        row.ActorCustomerNo,
		CustomerNo:             row.CustomerNo,
		OperationCode:          row.OperationCode,
		TargetType:             row.TargetType,
		TargetNo:               row.TargetNo,
		ResultCode:             row.ResultCode,
		ReasonCode:             row.ReasonCode,
		RiskCode:               row.RiskCode,
		BeforeSnapshot:         json.RawMessage(row.BeforeSnapshot),
		AfterSnapshot:          json.RawMessage(row.AfterSnapshot),
		ContextCore:            contextCore,
		RequestID:              row.RequestId,
		TraceID:                row.TraceId,
		CorrelationNo:          row.CorrelationNo,
		IPDigest:               normalizePersistedOptionalDigest(row.IpDigest),
		UserAgentDigest:        normalizePersistedOptionalDigest(row.UserAgentDigest),
		NumberType:             reservation.NumberType,
		ReservationScopeDigest: reservation.ReservationScopeDigest,
		IdempotencyKeyDigest:   reservation.IdempotencyKeyDigest,
		OwnerType:              reservation.OwnerType,
		OwnerInternalID:        reservation.OwnerInternalId,
		OwnerBusinessNo:        reservation.BusinessNo,
		OwnerAssignedAt:        canonicalIntegrityTime(*reservation.AssignedAt),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", gerror.Wrap(err, "encode persisted audit integrity payload")
	}
	return keyedDigestWithKey(key, rowIntegrityDomain, string(encoded))
}

// verifyPersistedEvent authenticates the strict stored envelopes before any
// evidence is projected or compared for replay.
func (s *serviceImpl) verifyPersistedEvent(
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
) (*verifiedEvidence, error) {
	if row == nil || row.OccurredAt == nil || reservation == nil || reservation.AssignedAt == nil {
		return nil, gerror.New("persisted audit event is incomplete")
	}
	before, err := s.parseStoredStateEnvelope(row.BeforeSnapshot)
	if err != nil {
		return nil, err
	}
	after, err := s.parseStoredStateEnvelope(row.AfterSnapshot)
	if err != nil {
		return nil, err
	}
	contextSnapshot, err := s.parseStoredContextEnvelope(row.ContextSnapshot)
	if err != nil {
		return nil, err
	}
	key, exists := s.integrityKeys[contextSnapshot.integrity.KeyID]
	if !exists {
		return nil, gerror.New("persisted audit integrity key is unavailable")
	}
	rowForMAC := *row
	rowForMAC.BeforeSnapshot = string(before.original)
	rowForMAC.AfterSnapshot = string(after.original)
	expected, err := rowIntegrityHMAC(key, &rowForMAC, reservation, contextSnapshot.originalCore)
	if err != nil {
		return nil, err
	}
	if !equalHMAC(expected, contextSnapshot.integrity.RowHMAC) {
		return nil, gerror.New("persisted audit row integrity verification failed")
	}
	replayKey, exists := s.replayKeys[contextSnapshot.integrity.ReplayKeyID]
	if !exists {
		return nil, gerror.New("persisted audit replay key is unavailable")
	}
	if err = s.validatePersistedSemantics(
		row,
		reservation,
		contextSnapshot,
		replayKey,
	); err != nil {
		return nil, err
	}
	return &verifiedEvidence{
		before:  before.redacted,
		after:   after.redacted,
		context: contextSnapshot.redacted,
		parsed:  contextSnapshot,
	}, nil
}

// parseStoredStateEnvelope enforces the complete state snapshot schema and
// returns both authenticated canonical input and defensively redacted output.
func (s *serviceImpl) parseStoredStateEnvelope(value string) (storedStateEnvelope, error) {
	object, err := s.normalizeObject(json.RawMessage(value))
	if err != nil || !hasExactKeys(object.originalObject, "schema_version", "data") {
		return storedStateEnvelope{}, gerror.New("persisted audit state envelope is invalid")
	}
	version, ok := strictJSONInt(object.originalObject["schema_version"])
	data, dataOK := object.originalObject["data"].(map[string]any)
	redactedData, redactedOK := object.redacted["data"].(map[string]any)
	if !ok || version != snapshotSchemaVersion || !dataOK || !redactedOK {
		return storedStateEnvelope{}, gerror.New("persisted audit state envelope schema is invalid")
	}
	original, err := json.Marshal(stateEnvelope{SchemaVersion: version, Data: data})
	if err != nil {
		return storedStateEnvelope{}, gerror.Wrap(err, "canonicalize persisted audit state")
	}
	redacted, err := json.Marshal(stateEnvelope{SchemaVersion: version, Data: redactedData})
	if err != nil {
		return storedStateEnvelope{}, gerror.Wrap(err, "redact persisted audit state")
	}
	return storedStateEnvelope{original: original, redacted: redacted}, nil
}

// parseStoredContextEnvelope enforces every context, security, and integrity
// field without accepting unknown or weakly typed members.
func (s *serviceImpl) parseStoredContextEnvelope(value string) (storedContextEnvelope, error) {
	object, err := s.normalizeObject(json.RawMessage(value))
	if err != nil || !hasExactKeys(object.originalObject, "schema_version", "data", "security", "integrity") {
		return storedContextEnvelope{}, gerror.New("persisted audit context envelope is invalid")
	}
	version, versionOK := strictJSONInt(object.originalObject["schema_version"])
	data, dataOK := object.originalObject["data"].(map[string]any)
	redactedData, redactedOK := object.redacted["data"].(map[string]any)
	securityMap, securityOK := object.originalObject["security"].(map[string]any)
	integrityMap, integrityOK := object.originalObject["integrity"].(map[string]any)
	if !versionOK || version != snapshotSchemaVersion || !dataOK || !redactedOK || !securityOK || !integrityOK {
		return storedContextEnvelope{}, gerror.New("persisted audit context envelope schema is invalid")
	}
	security, err := parseSecuritySnapshot(securityMap)
	if err != nil {
		return storedContextEnvelope{}, err
	}
	integrity, err := parseIntegritySnapshot(integrityMap)
	if err != nil {
		return storedContextEnvelope{}, err
	}
	originalCore, err := encodeContextCore(data, security, integrity)
	if err != nil {
		return storedContextEnvelope{}, err
	}
	redacted, err := json.Marshal(contextEnvelope{
		SchemaVersion: version,
		Data:          redactedData,
		Security:      security,
		Integrity:     integrity,
	})
	if err != nil {
		return storedContextEnvelope{}, gerror.Wrap(err, "redact persisted audit context")
	}
	return storedContextEnvelope{
		originalCore: originalCore,
		redacted:     redacted,
		data:         data,
		security:     security,
		integrity:    integrity,
	}, nil
}

// parseSecuritySnapshot converts an exact generic object into its strict type.
func parseSecuritySnapshot(value map[string]any) (securitySnapshot, error) {
	if !hasExactKeys(
		value,
		"entry_kind",
		"actor_audience_code",
		"actor_reference_hmac",
		"data_scope",
		"delegated",
		"required_permission",
		"required_granted",
		"wildcard_granted",
		"platform_admin",
		"permission_snapshot_hmac",
		"platform_bypass",
	) {
		return securitySnapshot{}, gerror.New("persisted audit security schema is invalid")
	}
	dataScope, dataScopeOK := strictJSONInt(value["data_scope"])
	entryKind, entryKindOK := value["entry_kind"].(string)
	audience, audienceOK := value["actor_audience_code"].(string)
	actorReference, actorReferenceOK := value["actor_reference_hmac"].(string)
	delegated, delegatedOK := value["delegated"].(bool)
	requiredPermission, permissionOK := value["required_permission"].(string)
	requiredGranted, requiredGrantedOK := value["required_granted"].(bool)
	wildcardGranted, wildcardGrantedOK := value["wildcard_granted"].(bool)
	platformAdmin, platformAdminOK := value["platform_admin"].(bool)
	permissionHMAC, permissionHMACOK := value["permission_snapshot_hmac"].(string)
	platformBypass, platformBypassOK := value["platform_bypass"].(bool)
	if !dataScopeOK || !entryKindOK || !audienceOK || !actorReferenceOK ||
		!delegatedOK || !permissionOK || !requiredGrantedOK || !wildcardGrantedOK ||
		!platformAdminOK || !permissionHMACOK || !platformBypassOK {
		return securitySnapshot{}, gerror.New("persisted audit security field type is invalid")
	}
	return securitySnapshot{
		EntryKind:              entryKind,
		ActorAudienceCode:      audience,
		ActorReferenceHMAC:     actorReference,
		DataScope:              dataScope,
		Delegated:              delegated,
		RequiredPermission:     requiredPermission,
		RequiredGranted:        requiredGranted,
		WildcardGranted:        wildcardGranted,
		PlatformAdmin:          platformAdmin,
		PermissionSnapshotHMAC: permissionHMAC,
		PlatformBypass:         platformBypass,
	}, nil
}

// parseIntegritySnapshot converts the exact versioned integrity object.
func parseIntegritySnapshot(value map[string]any) (integritySnapshot, error) {
	if !hasExactKeys(
		value,
		"version",
		"key_id",
		"replay_key_id",
		"raw_replay_hmac",
		"row_hmac",
	) {
		return integritySnapshot{}, gerror.New("persisted audit integrity schema is invalid")
	}
	version, versionOK := strictJSONInt(value["version"])
	keyID, keyOK := value["key_id"].(string)
	replayKeyID, replayKeyOK := value["replay_key_id"].(string)
	rawReplayHMAC, replayOK := value["raw_replay_hmac"].(string)
	rowHMAC, rowOK := value["row_hmac"].(string)
	if !versionOK || version != integritySchemaVersion || !keyOK ||
		!validPersistedStableCode(keyID, maximumIntegrityKeyIDRunes, true) ||
		!replayKeyOK || !validPersistedStableCode(replayKeyID, maximumIntegrityKeyIDRunes, true) ||
		!replayOK || !isCanonicalHMAC(rawReplayHMAC) ||
		!rowOK || !isCanonicalHMAC(rowHMAC) {
		return integritySnapshot{}, gerror.New("persisted audit integrity field is invalid")
	}
	return integritySnapshot{
		Version:       version,
		KeyID:         keyID,
		ReplayKeyID:   replayKeyID,
		RawReplayHMAC: rawReplayHMAC,
		RowHMAC:       rowHMAC,
	}, nil
}

// validatePersistedSemantics checks relational and actor invariants only after
// the row has passed constant-time MAC verification.
func (s *serviceImpl) validatePersistedSemantics(
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
	contextSnapshot storedContextEnvelope,
	replayKey []byte,
) error {
	security := contextSnapshot.security
	if err := validatePersistedRowFields(row); err != nil {
		return err
	}
	if err := validatePersistedReservation(row, reservation); err != nil {
		return err
	}
	if err := validatePersistedSecurityFields(row, security); err != nil {
		return err
	}
	authority, err := verifyPersistedPermissionEvidence(security, replayKey)
	if err != nil {
		return err
	}
	return validatePersistedActor(row, security, authority)
}

// validatePersistedRowFields checks the immutable business evidence scalars.
func validatePersistedRowFields(row *entity.BusinessAuditEvent) error {
	if row.Id <= 0 || row.TenantId != idccap.DefaultTenantID ||
		row.OccurredAt.IsZero() || !validAuditNumber(row.AuditNo) ||
		!validResult(ResultCode(row.ResultCode)) || !validRisk(RiskCode(row.RiskCode)) ||
		!validPersistedStableCode(row.OperationCode, maximumIdentifierRunes, true) ||
		!validPersistedStableCode(row.TargetType, 64, true) ||
		!validReferenceText(row.TargetNo, 64, true) {
		return gerror.New("persisted audit row identity is invalid")
	}
	if !validReferenceText(row.ActorCustomerNo, 40, false) ||
		!validReferenceText(row.CustomerNo, 40, false) ||
		!validPersistedStableCode(row.ReasonCode, maximumIdentifierRunes, false) ||
		(ResultCode(row.ResultCode) != ResultSucceeded && row.ReasonCode == "") ||
		!validReferenceText(row.RequestId, maximumIdentifierRunes, false) ||
		!validReferenceText(row.TraceId, maximumIdentifierRunes, false) ||
		!validReferenceText(row.CorrelationNo, 64, false) {
		return gerror.New("persisted audit row business field is invalid")
	}
	if !validOptionalPersistedHMAC(row.IpDigest) ||
		!validOptionalPersistedHMAC(row.UserAgentDigest) {
		return gerror.New("persisted audit request digest is invalid")
	}
	return nil
}

// validatePersistedReservation checks the business-number and relational owner
// binding used alongside the pre-insert MAC owner statement.
func validatePersistedReservation(
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
) error {
	if reservation.TenantId != idccap.DefaultTenantID ||
		reservation.NumberType != businessNumberTypeAudit ||
		reservation.BusinessNo != row.AuditNo ||
		reservation.OwnerType != businessNumberOwnerAudit ||
		reservation.OwnerInternalId != row.Id ||
		!reservation.AssignedAt.Equal(*row.OccurredAt) ||
		!isCanonicalDigest(reservation.ReservationScopeDigest) ||
		!isCanonicalDigest(reservation.IdempotencyKeyDigest) {
		return gerror.New("persisted audit number owner binding is invalid")
	}
	return nil
}

// validatePersistedSecurityFields checks strict security metadata independent
// of actor-type-specific subject rules.
func validatePersistedSecurityFields(
	row *entity.BusinessAuditEvent,
	security securitySnapshot,
) error {
	if (security.EntryKind != entryKindTransition && security.EntryKind != entryKindObservation) ||
		security.ActorAudienceCode != row.ActorAudienceCode || security.DataScope < 0 ||
		!validAuditPermission(security.RequiredPermission) {
		return gerror.New("persisted audit security evidence is invalid")
	}
	return nil
}

// verifyPersistedPermissionEvidence authenticates the replay-key permission
// evidence after the enclosing row MAC has already passed.
func verifyPersistedPermissionEvidence(
	security securitySnapshot,
	replayKey []byte,
) (authorizationEvidence, error) {
	authority := authorizationEvidence{
		RequiredPermission: security.RequiredPermission,
		RequiredGranted:    security.RequiredGranted,
		WildcardGranted:    security.WildcardGranted,
		PlatformAdmin:      security.PlatformAdmin,
	}
	authorityJSON, err := json.Marshal(authority)
	if err != nil {
		return authorizationEvidence{}, gerror.Wrap(err, "encode persisted audit permission evidence")
	}
	expectedPermissionHMAC, err := keyedDigestWithKey(
		replayKey,
		"audit-permissions-v1",
		string(authorityJSON),
	)
	if err != nil {
		return authorizationEvidence{}, err
	}
	if !equalHMAC(expectedPermissionHMAC, security.PermissionSnapshotHMAC) {
		return authorizationEvidence{}, gerror.New("persisted audit permission integrity verification failed")
	}
	return authority, nil
}

// validatePersistedActor enforces user, system, external, audience, and subject
// invariants after row and permission MAC verification.
func validatePersistedActor(
	row *entity.BusinessAuditEvent,
	security securitySnapshot,
	authority authorizationEvidence,
) error {
	switch ActorType(row.ActorType) {
	case ActorUser:
		if row.ActorUserId <= 0 || row.ActingUserId < 0 || row.ActingUserId == row.ActorUserId ||
			security.Delegated != (row.ActingUserId > 0) || security.ActorReferenceHMAC != "" ||
			(principal.AudienceCode(row.ActorAudienceCode) != principal.AudienceCustomer &&
				principal.AudienceCode(row.ActorAudienceCode) != principal.AudienceStaff) {
			return gerror.New("persisted user audit actor is invalid")
		}
	case ActorSystem, ActorExternal:
		if row.ActorUserId != 0 || row.ActingUserId != 0 || row.ActorAudienceCode != "" ||
			row.ActorCustomerNo != "" || security.DataScope != 0 || security.Delegated ||
			security.PlatformBypass || !isCanonicalHMAC(security.ActorReferenceHMAC) ||
			security.RequiredPermission != "" || authority.granted() {
			return gerror.New("persisted non-user audit actor is invalid")
		}
	default:
		return gerror.New("persisted audit actor type is invalid")
	}
	switch principal.AudienceCode(row.ActorAudienceCode) {
	case principal.AudienceCustomer:
		if row.ActorCustomerNo == "" || row.ActorCustomerNo != row.CustomerNo ||
			security.RequiredPermission != "" || authority.granted() {
			return gerror.New("persisted customer audit subject is invalid")
		}
	case principal.AudienceStaff:
		actor := TrustedActor{DataScope: security.DataScope}
		if row.ActorCustomerNo != "" || !idccap.IsAdminPermission(security.RequiredPermission) ||
			!isSupportedStaffDataScope(actor, authority) ||
			(ResultCode(row.ResultCode) != ResultDenied && !authority.granted()) {
			return gerror.New("persisted staff audit subject is invalid")
		}
	}
	return nil
}

// validOptionalPersistedHMAC accepts an absent padded CHAR value or one
// canonical keyed digest.
func validOptionalPersistedHMAC(value string) bool {
	normalized := normalizePersistedOptionalDigest(value)
	return normalized == "" || isCanonicalHMAC(normalized)
}

// hasExactKeys rejects both missing and unknown envelope fields.
func hasExactKeys(value map[string]any, keys ...string) bool {
	if len(value) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, exists := value[key]; !exists {
			return false
		}
	}
	return true
}

// strictJSONInt accepts only canonical base-10 JSON integers in int range.
func strictJSONInt(value any) (int, bool) {
	var raw string
	switch number := value.(type) {
	case canonicalNumberValue:
		raw = number.lexical
	case json.Number:
		raw = number.String()
	default:
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || strconv.Itoa(parsed) != raw {
		return 0, false
	}
	return parsed, true
}

// equalHMAC compares canonical SHA-256 MACs in constant time.
func equalHMAC(left string, right string) bool {
	leftBytes, leftErr := hex.DecodeString(left)
	rightBytes, rightErr := hex.DecodeString(right)
	if leftErr != nil || rightErr != nil || len(leftBytes) != 32 || len(rightBytes) != 32 {
		return false
	}
	return hmac.Equal(leftBytes, rightBytes)
}

// isCanonicalHMAC validates the lowercase fixed-width representation used by
// all keyed audit digests.
func isCanonicalHMAC(value string) bool {
	return isCanonicalDigest(value)
}

// isCanonicalDigest validates one lowercase fixed-width SHA-256 encoding.
func isCanonicalDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}

// validAuditNumber validates the opaque lower-case 128-bit audit identifier.
func validAuditNumber(value string) bool {
	if len(value) != len(auditNumberPrefix)+32 ||
		value[:len(auditNumberPrefix)] != auditNumberPrefix {
		return false
	}
	encoded := value[len(auditNumberPrefix):]
	decoded, err := hex.DecodeString(encoded)
	return err == nil && len(decoded) == 16 && hex.EncodeToString(decoded) == encoded
}

// canonicalIntegrityTime normalizes timestamps before integrity encoding.
func canonicalIntegrityTime(value time.Time) string {
	return databaseTime(value).Format(time.RFC3339Nano)
}

// normalizePersistedOptionalDigest maps CHAR-column padding for an absent
// optional digest back to its canonical logical empty value.
func normalizePersistedOptionalDigest(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}
