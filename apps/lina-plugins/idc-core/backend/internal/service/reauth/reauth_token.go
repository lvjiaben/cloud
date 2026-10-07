// This file implements canonical, signed, session-bound proof encoding and verification.

package reauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	// Proof wire constants are versioned and bounded independently of caller input.
	proofWirePrefix       = "r1"
	proofSchemaVersion    = 1
	maximumClaimsBytes    = 2048
	maximumEncodedSegment = 8192
)

// proofClaims is the canonical signed payload. It contains no raw session ID or business target.
type proofClaims struct {
	// Version identifies the claims schema.
	Version int `json:"v"`
	// SessionDigest binds the proof without exposing the raw session identifier.
	SessionDigest string `json:"sid"`
	// UserID is the authenticated host identity.
	UserID int `json:"uid"`
	// TenantID is the single-merchant tenant boundary.
	TenantID int `json:"tid"`
	// LoginAt is the host session login time in Unix milliseconds.
	LoginAt int64 `json:"lat"`
	// IssuedAt is the proof issue time in Unix milliseconds.
	IssuedAt int64 `json:"iat"`
	// ExpiresAt is the hard proof deadline in Unix milliseconds.
	ExpiresAt int64 `json:"exp"`
	// Purpose binds one high-risk operation family.
	Purpose Purpose `json:"purpose"`
	// TargetDigest binds the immutable operation target or preview.
	TargetDigest string `json:"target"`
	// Nonce provides the cluster-wide single-use replay key.
	Nonce string `json:"nonce"`
	// SessionConfirmation distinguishes a confirm click from a recent-login proof.
	SessionConfirmation bool `json:"sc,omitempty"`
}

// Issue validates a newly created host session and returns one opaque signed proof.
func (s *serviceImpl) Issue(ctx context.Context, binding Binding) (*ProofContext, error) {
	return s.issueProof(ctx, binding, true)
}

// IssueSessionConfirmation signs one exact action confirmation for the
// current active session without requiring a recently created login session.
func (s *serviceImpl) IssueSessionConfirmation(ctx context.Context,
	binding Binding) (*ProofContext, error) {
	if binding.Purpose != PurposeServerOperation {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return s.issueProof(ctx, binding, false)
}

func (s *serviceImpl) issueProof(ctx context.Context, binding Binding,
	requireFreshLogin bool) (*ProofContext, error) {
	if ctx == nil {
		return nil, bizerr.NewCode(CodeRecentLoginRequired)
	}
	binding, err := normalizeBinding(binding)
	if err != nil {
		return nil, err
	}
	now := databaseTime(s.now())
	if now.IsZero() {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	identity, session, err := s.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	loginAt := databaseTime(*session.LoginAt)
	if loginAt.After(now.Add(s.config.AllowedClockSkew)) ||
		(requireFreshLogin && now.Sub(loginAt) > s.config.FreshLoginWindow) {
		return nil, bizerr.NewCode(CodeRecentLoginRequired)
	}
	expiresAt := now.Add(s.config.ProofTTL)
	if requireFreshLogin {
		loginDeadline := loginAt.Add(s.config.FreshLoginWindow)
		if loginDeadline.Before(expiresAt) {
			expiresAt = loginDeadline
		}
	}
	if !expiresAt.After(now) {
		return nil, bizerr.NewCode(CodeRecentLoginRequired)
	}
	nonce := make([]byte, nonceBytes)
	if _, err = io.ReadFull(s.random, nonce); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	claims := proofClaims{
		Version:             proofSchemaVersion,
		SessionDigest:       digestSessionID(identity.TokenID),
		UserID:              identity.UserID,
		TenantID:            identity.TenantID,
		LoginAt:             loginAt.UnixMilli(),
		IssuedAt:            now.UnixMilli(),
		ExpiresAt:           expiresAt.UnixMilli(),
		Purpose:             binding.Purpose,
		TargetDigest:        binding.TargetDigest,
		Nonce:               base64.RawURLEncoding.EncodeToString(nonce),
		SessionConfirmation: !requireFreshLogin,
	}
	payload, err := json.Marshal(claims)
	if err != nil || len(payload) > maximumClaimsBytes {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	payloadSegment := base64.RawURLEncoding.EncodeToString(payload)
	keyID, err := s.signer.ActiveKeyID(ctx)
	if err != nil || !validKeyID(keyID) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	message := proofMessage(keyID, payloadSegment)
	signature, err := s.signer.Sign(ctx, keyID, message)
	if err != nil ||
		len(signature) < minimumSignatureBytes || len(signature) > maximumSignatureBytes {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	token := strings.Join([]string{
		proofWirePrefix,
		keyID,
		payloadSegment,
		base64.RawURLEncoding.EncodeToString(signature),
	}, ".")
	if len(token) > s.config.MaximumTokenSize {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &ProofContext{Token: token, ExpiresAt: expiresAt}, nil
}

// Consume verifies every proof binding before atomically claiming its nonce in
// the durable replay store. The proof is intentionally burned before returning
// it to a dangerous operation; callers must never restore a consumed proof.
func (s *serviceImpl) Consume(ctx context.Context, token string, expected Binding) (*Proof, error) {
	if ctx == nil {
		return nil, bizerr.NewCode(CodeProofRejected)
	}
	expected, err := normalizeBinding(expected)
	if err != nil {
		return nil, err
	}
	claims, keyID, message, signature, err := s.parseToken(token)
	if err != nil {
		return nil, err
	}
	verified, err := s.signer.Verify(ctx, keyID, message, signature)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if !verified || claims.Purpose != expected.Purpose || claims.TargetDigest != expected.TargetDigest {
		return nil, bizerr.NewCode(CodeProofRejected)
	}
	now := databaseTime(s.now())
	if err = s.validateClaimsAt(claims, now); err != nil {
		return nil, err
	}
	identity, session, err := s.currentSession(ctx)
	if err != nil {
		if bizerr.Is(err, CodeRecentLoginRequired) {
			return nil, bizerr.NewCode(CodeProofRejected)
		}
		return nil, err
	}
	loginAt := databaseTime(*session.LoginAt)
	if identity.UserID != claims.UserID || identity.TenantID != claims.TenantID ||
		digestSessionID(identity.TokenID) != claims.SessionDigest || loginAt.UnixMilli() != claims.LoginAt {
		return nil, bizerr.NewCode(CodeProofRejected)
	}
	expiresAt := time.UnixMilli(claims.ExpiresAt).UTC()
	if !expiresAt.After(now) {
		return nil, bizerr.NewCode(CodeProofRejected)
	}
	claimed, err := s.replay.Claim(ctx, ReplayClaim{
		NonceDigest: digestNonce(claims.Nonce), Purpose: claims.Purpose,
		TargetDigest: claims.TargetDigest, SessionDigest: claims.SessionDigest,
		UserID: claims.UserID, TenantID: claims.TenantID,
		LoginAt: loginAt, IssuedAt: time.UnixMilli(claims.IssuedAt).UTC(),
		ExpiresAt:           expiresAt,
		SessionConfirmation: claims.SessionConfirmation,
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if !claimed {
		return nil, bizerr.NewCode(CodeProofRejected)
	}
	return &Proof{
		Purpose: claims.Purpose, TargetDigest: claims.TargetDigest,
		UserID: claims.UserID, TenantID: claims.TenantID,
		LoginAt: loginAt, IssuedAt: time.UnixMilli(claims.IssuedAt).UTC(),
		ExpiresAt: expiresAt, SessionDigest: claims.SessionDigest,
	}, nil
}

// parseToken decodes only the canonical r1 wire form and rejects aliases or unknown claims.
func (s *serviceImpl) parseToken(token string) (proofClaims, string, []byte, []byte, error) {
	if token == "" || token != strings.TrimSpace(token) || len(token) > s.config.MaximumTokenSize {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != proofWirePrefix || !validKeyID(parts[1]) ||
		len(parts[2]) == 0 || len(parts[2]) > maximumEncodedSegment ||
		len(parts[3]) == 0 || len(parts[3]) > maximumEncodedSegment {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(payload) > maximumClaimsBytes ||
		base64.RawURLEncoding.EncodeToString(payload) != parts[2] {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var claims proofClaims
	if err = decoder.Decode(&claims); err != nil || !jsonDecoderAtEnd(decoder) {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !bytes.Equal(canonical, payload) {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(signature) < minimumSignatureBytes || len(signature) > maximumSignatureBytes ||
		base64.RawURLEncoding.EncodeToString(signature) != parts[3] {
		return proofClaims{}, "", nil, nil, bizerr.NewCode(CodeProofRejected)
	}
	return claims, parts[1], proofMessage(parts[1], parts[2]), signature, nil
}

// validateClaimsAt checks the closed schema and all absolute time relationships.
func (s *serviceImpl) validateClaimsAt(claims proofClaims, now time.Time) error {
	if now.IsZero() || claims.Version != proofSchemaVersion || claims.UserID <= 0 ||
		claims.TenantID != idccap.DefaultTenantID || !isLowerHexDigest(claims.SessionDigest) ||
		!validPurpose(claims.Purpose) || !isLowerHexDigest(claims.TargetDigest) ||
		!validNonce(claims.Nonce) {
		return bizerr.NewCode(CodeProofRejected)
	}
	loginAt := time.UnixMilli(claims.LoginAt).UTC()
	issuedAt := time.UnixMilli(claims.IssuedAt).UTC()
	expiresAt := time.UnixMilli(claims.ExpiresAt).UTC()
	if loginAt.After(issuedAt.Add(s.config.AllowedClockSkew)) ||
		issuedAt.After(now.Add(s.config.AllowedClockSkew)) ||
		!expiresAt.After(now) || !expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > s.config.ProofTTL {
		return bizerr.NewCode(CodeProofRejected)
	}
	if claims.SessionConfirmation {
		if claims.Purpose != PurposeServerOperation {
			return bizerr.NewCode(CodeProofRejected)
		}
	} else if issuedAt.Sub(loginAt) > s.config.FreshLoginWindow ||
		expiresAt.After(loginAt.Add(s.config.FreshLoginWindow)) {
		return bizerr.NewCode(CodeProofRejected)
	}
	return nil
}

// currentSession cross-checks the request identity against the host's authoritative
// online-session projection and rejects all delegated or impersonated contexts.
func (s *serviceImpl) currentSession(ctx context.Context) (identity struct {
	TokenID  string
	UserID   int
	TenantID int
}, session *sessioncap.SessionInfo, err error) {
	current := s.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.TokenID == "" || len(current.TokenID) > maximumSessionIDBytes ||
		current.IsImpersonation || current.ActingAsTenant || current.ActingUserID > 0 {
		return identity, nil, bizerr.NewCode(CodeRecentLoginRequired)
	}
	session, err = s.sessionSvc.Current(ctx)
	if err != nil {
		return identity, nil, bizerr.NewCode(CodeUnavailable)
	}
	if session == nil || session.LoginAt == nil || string(session.ID) != current.TokenID ||
		session.UserID != strconv.Itoa(current.UserID) || string(session.TenantID) != strconv.Itoa(current.TenantID) {
		return identity, nil, bizerr.NewCode(CodeRecentLoginRequired)
	}
	identity.TokenID = current.TokenID
	identity.UserID = current.UserID
	identity.TenantID = current.TenantID
	return identity, session, nil
}

// proofMessage binds wire version, signing-key identity, and exact canonical payload.
func proofMessage(keyID string, payloadSegment string) []byte {
	return []byte("idc-reauth\x00" + proofWirePrefix + "\x00" + keyID + "\x00" + payloadSegment)
}

// digestSessionID creates the non-reversible session binding carried by a proof.
func digestSessionID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// digestNonce returns a fixed-size non-reversible replay key. Nonces contain
// 192 bits of cryptographic entropy, so an unkeyed digest does not expose the
// client-carried nonce while remaining deterministic across instances.
func digestNonce(value string) string {
	digest := sha256.Sum256([]byte("idc-reauth-nonce\x00" + value))
	return hex.EncodeToString(digest[:])
}

// validNonce checks canonical base64url encoding and the full entropy length.
func validNonce(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == nonceBytes && base64.RawURLEncoding.EncodeToString(decoded) == value
}

// jsonDecoderAtEnd rejects any second or trailing non-whitespace JSON value.
func jsonDecoderAtEnd(decoder *json.Decoder) bool {
	var trailing any
	return decoder.Decode(&trailing) == io.EOF
}
