// Package onboarding owns creation of short-lived verified customer intents.
package onboarding

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/onboardingcap"
)

const intentLifetime = 30 * time.Minute

type Service struct {
	database gdb.DB
	key      []byte
}

func New(database gdb.DB, key []byte) (*Service, error) {
	if database == nil || len(key) != sha256.Size {
		return nil, gerror.New("customer onboarding intent dependencies are invalid")
	}
	return &Service{database: database, key: append([]byte(nil), key...)}, nil
}

type intentRow struct {
	IntentNo      string    `json:"intent_no"`
	RequestDigest string    `json:"request_digest"`
	ExpiresAt     time.Time `json:"expires_at"`
	StatusCode    string    `json:"status_code"`
}

func (s *Service) CreateVerifiedIntent(ctx context.Context, request onboardingcap.CreateVerifiedIntentRequest) (*onboardingcap.VerifiedIntent, error) {
	if err := onboardingcap.ValidateCreateRequest(request); err != nil {
		return nil, err
	}
	token := s.token(request.OperationDigest)
	var result *onboardingcap.VerifiedIntent
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.database.GetValue(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "customer-onboarding:"+request.OperationDigest); err != nil {
			return err
		}
		var existing *intentRow
		if err := s.database.GetScan(ctx, &existing, `SELECT intent_no,request_digest,expires_at,status_code FROM plugin_idc_core_customer_onboarding_intent WHERE tenant_id=0 AND operation_digest=? FOR UPDATE`, request.OperationDigest); err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestDigest != request.RequestDigest {
				return gerror.New("customer onboarding idempotency conflict")
			}
			result = &onboardingcap.VerifiedIntent{IntentNo: existing.IntentNo, Token: token, ExpiresAt: existing.ExpiresAt.UTC(), Reused: true}
			return nil
		}
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return err
		}
		intentNo := "ONB_" + hex.EncodeToString(raw[:])
		expiresAt := request.VerifiedAt.UTC().Add(intentLifetime)
		if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_customer_onboarding_intent SET status_code='cancelled',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND status_code='verified' AND browser_nonce_digest=? AND normalized_username_hash=?`, request.BrowserDigest, request.UsernameDigest); err != nil {
			return err
		}
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_onboarding_intent (tenant_id,intent_no,token_digest,browser_nonce_digest,normalized_username_hash,contact_channel_code,contact_hash,customer_type_code,status_code,return_path,verified_contact_at,expires_at,operation_digest,request_digest) VALUES (0,?,?,?,?,?,?,?,'verified',?,?,?,?,?)`, intentNo, digest(token), request.BrowserDigest, request.UsernameDigest, request.ContactChannel, request.ContactDigest, request.CustomerType, request.ReturnPath, request.VerifiedAt.UTC(), expiresAt, request.OperationDigest, request.RequestDigest)
		if err != nil {
			return err
		}
		result = &onboardingcap.VerifiedIntent{IntentNo: intentNo, Token: token, ExpiresAt: expiresAt}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, onboardingcap.ValidateResult(*result)
}

func (s *Service) token(operationDigest string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("customer-onboarding-token-v1\x00" + strings.TrimSpace(operationDigest)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
