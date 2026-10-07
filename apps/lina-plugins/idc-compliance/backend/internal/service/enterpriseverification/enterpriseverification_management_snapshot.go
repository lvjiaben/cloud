// This file reads authenticated legal snapshots and returns masked projections.

package enterpriseverification

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
)

// legalSnapshot owns the decrypted legal fields and must be cleared after projection.
type legalSnapshot struct {
	legalName                         []byte
	registrationCountryCode           string
	registrationIdentifier            []byte
	legalRepresentativeName           []byte
	legalRepresentativeIdentityNumber []byte
	agentName                         []byte
	agentIdentityNumber               []byte
	industryCode                      string
	applicantRole                     ApplicantRole
	materialNos                       []string
}

// clear wipes every decrypted identity byte slice owned by the snapshot.
func (p *legalSnapshot) clear() {
	if p == nil {
		return
	}
	clearBytes(p.legalName)
	clearBytes(p.registrationIdentifier)
	clearBytes(p.legalRepresentativeName)
	clearBytes(p.legalRepresentativeIdentityNumber)
	clearBytes(p.agentName)
	clearBytes(p.agentIdentityNumber)
}

// loadLegalSnapshot authenticates and decrypts one private server-authoritative snapshot.
func (s *managementServiceImpl) loadLegalSnapshot(
	ctx context.Context,
	row *entity.VerificationCase,
) (*legalSnapshot, error) {
	if row == nil || row.EncryptionKeyNo == nil || row.LegalProfileSnapshotRef == nil ||
		row.LegalProfileSha256 == nil || row.SnapshotStoredByteSize <= 0 ||
		row.SnapshotStoredByteSize > maximumSnapshotPlaintextBytes+1024 {
		return nil, gerror.New("enterprise verification legal snapshot is unavailable")
	}
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: *row.LegalProfileSnapshotRef})
	if err != nil || result == nil || !result.Found || result.Object == nil || result.Body == nil {
		return nil, gerror.New("enterprise verification legal snapshot is unavailable")
	}
	if result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != row.SnapshotStoredByteSize ||
		result.Object.ContentType != "application/octet-stream" {
		if closeErr := result.Body.Close(); closeErr != nil {
			return nil, gerror.Wrap(closeErr, "close inconsistent enterprise legal snapshot")
		}
		return nil, gerror.New("enterprise verification legal snapshot is inconsistent")
	}
	envelope, readErr := io.ReadAll(io.LimitReader(result.Body, row.SnapshotStoredByteSize+1))
	closeErr := result.Body.Close()
	if readErr != nil || closeErr != nil || int64(len(envelope)) != row.SnapshotStoredByteSize ||
		sha256Hex(envelope) != *row.LegalProfileSha256 {
		clearBytes(envelope)
		return nil, gerror.New("enterprise verification legal snapshot integrity check failed")
	}
	defer clearBytes(envelope)
	var plaintext []byte
	err = accessEnterpriseKey(ctx, s.keyAccessor, *row.EncryptionKeyNo, func(key []byte) error {
		var decryptErr error
		plaintext, decryptErr = decryptEnvelope(
			key, envelope, "snapshot:"+row.CaseNo, maximumSnapshotPlaintextBytes,
		)
		return decryptErr
	})
	if err != nil {
		clearBytes(plaintext)
		return nil, err
	}
	defer clearBytes(plaintext)
	profile, err := parseLegalSnapshot(plaintext, row.CaseNo)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

// parseLegalSnapshot decodes the closed LEP1 envelope payload with strict bounds.
func parseLegalSnapshot(value []byte, caseNo string) (*legalSnapshot, error) {
	if len(value) < 4 || !bytes.Equal(value[:4], []byte{'L', 'E', 'P', '1'}) {
		return nil, gerror.New("enterprise verification legal snapshot is invalid")
	}
	offset := 4
	fields := make([][]byte, 0, 9+maximumSelectedCount)
	for offset < len(value) {
		if len(fields) >= 9+maximumSelectedCount || len(value)-offset < 4 {
			clearByteFields(fields)
			return nil, gerror.New("enterprise verification legal snapshot is invalid")
		}
		size := int(binary.BigEndian.Uint32(value[offset : offset+4]))
		offset += 4
		if size < 0 || size > maximumSnapshotPlaintextBytes || len(value)-offset < size {
			clearByteFields(fields)
			return nil, gerror.New("enterprise verification legal snapshot is invalid")
		}
		fields = append(fields, append([]byte(nil), value[offset:offset+size]...))
		offset += size
	}
	if len(fields) < 10 {
		clearByteFields(fields)
		return nil, gerror.New("enterprise verification legal snapshot is invalid")
	}
	profile := &legalSnapshot{
		legalName: fields[0], registrationCountryCode: string(fields[1]),
		registrationIdentifier: fields[2], legalRepresentativeName: fields[3],
		legalRepresentativeIdentityNumber: fields[4], agentName: fields[5],
		agentIdentityNumber: fields[6], industryCode: string(fields[7]),
		applicantRole: ApplicantRole(fields[8]),
	}
	for _, field := range fields[9:] {
		profile.materialNos = append(profile.materialNos, string(field))
		clearBytes(field)
	}
	legal, err := validateLegalProfile(SubmitInput{
		CaseNo: caseNo, LegalName: profile.legalName,
		RegistrationCountryCode:           profile.registrationCountryCode,
		RegistrationIdentifier:            profile.registrationIdentifier,
		LegalRepresentativeName:           profile.legalRepresentativeName,
		LegalRepresentativeIdentityNumber: profile.legalRepresentativeIdentityNumber,
		AgentName:                         profile.agentName, AgentIdentityNumber: profile.agentIdentityNumber,
		IndustryCode: profile.industryCode, ApplicantRole: profile.applicantRole,
		IdempotencyKey: "snapshot-validation-v1",
	})
	if err != nil {
		profile.clear()
		return nil, gerror.New("enterprise verification legal snapshot is invalid")
	}
	defer legal.clear()
	materialNos, err := normalizeMaterialNos(profile.materialNos)
	if err != nil {
		profile.clear()
		return nil, gerror.New("enterprise verification legal snapshot is invalid")
	}
	profile.materialNos = materialNos
	return profile, nil
}

// masked returns the minimum staff review projection without raw identifiers.
func (p *legalSnapshot) masked() MaskedLegalProfile {
	return MaskedLegalProfile{
		LegalName:               maskLegalName(p.legalName),
		RegistrationCountryCode: p.registrationCountryCode,
		RegistrationIdentifier:  maskRegistrationIdentifier(p.registrationIdentifier),
		LegalRepresentativeName: maskLegalName(p.legalRepresentativeName),
		LegalRepresentativeID:   maskIdentityDocument(p.legalRepresentativeIdentityNumber),
		AgentPresent:            len(p.agentName) > 0,
		AgentName:               maskLegalName(p.agentName),
		AgentIdentityNumber:     maskIdentityDocument(p.agentIdentityNumber),
		IndustryCode:            p.industryCode, ApplicantRoleCode: string(p.applicantRole),
	}
}

// materialSetMatches proves the reviewed snapshot and persisted evidence sets agree.
func (p *legalSnapshot) materialSetMatches(rows []*entity.EnterpriseMaterial) bool {
	if p == nil || len(p.materialNos) != len(rows) {
		return false
	}
	actual := make([]string, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			return false
		}
		actual = append(actual, row.MaterialNo)
	}
	sort.Strings(actual)
	return strings.Join(actual, "\x00") == strings.Join(p.materialNos, "\x00")
}

// maskIdentityDocument retains only the last four validated Unicode characters.
func maskIdentityDocument(value []byte) string {
	if len(value) == 0 || !utf8.Valid(value) {
		return ""
	}
	runes := []rune(string(value))
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}

// clearByteFields wipes all temporary decoded fields on parse failure.
func clearByteFields(values [][]byte) {
	for _, value := range values {
		clearBytes(value)
	}
}
