package challenge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-shared-services/backend/cap/challengecap"
	modeldo "lina-plugin-shared-services/backend/internal/model/do"
	"lina-plugin-shared-services/backend/internal/service/sharedcode"
	"lina-plugin-shared-services/backend/internal/service/sharedutil"
)

const challengeTable = "plugin_shared_services_challenge"

type challengeRow struct {
	ChallengeNo      string     `orm:"challenge_no"`
	CallerPluginID   string     `orm:"caller_plugin_id"`
	Purpose          string     `orm:"purpose"`
	BindingDigest    string     `orm:"binding_digest"`
	ProviderCode     string     `orm:"provider_code"`
	PresentationKind string     `orm:"presentation_kind"`
	AnswerSaltHex    string     `orm:"answer_salt_hex"`
	AnswerDigestHex  string     `orm:"answer_digest_hex"`
	Status           string     `orm:"status"`
	AttemptCount     int        `orm:"attempt_count"`
	ExpiresAt        time.Time  `orm:"expires_at"`
	ProofExpiresAt   *time.Time `orm:"proof_expires_at"`
}

type serviceImpl struct {
	database   gdb.DB
	config     Config
	httpClient *http.Client
}

// ServiceRuntime is the owner-composed runtime used only by plugin startup.
type ServiceRuntime = serviceImpl

type boundClient struct {
	service *serviceImpl
	caller  string
}

var _ challengecap.Service = (*boundClient)(nil)
var _ capregistry.Invoker = (*Invoker)(nil)

func New(database gdb.DB, config Config) (*serviceImpl, error) {
	if database == nil || (config.Provider != "local" && config.Provider != "turnstile") || config.RequestTimeout <= 0 {
		return nil, gerror.New("shared challenge service dependencies are invalid")
	}
	if config.Provider == "turnstile" && (config.TurnstileSiteKey == "" || config.TurnstileSecret == "" || config.TurnstileURL != turnstileVerifyURL) {
		return nil, gerror.New("shared challenge Turnstile configuration is invalid")
	}
	return &serviceImpl{database: database, config: config, httpClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: config.RequestTimeout}}, nil
}

func Bind(service *serviceImpl, caller string) (challengecap.Service, error) {
	if service == nil || caller == "" || caller == challengecap.OwnerPluginID {
		return nil, gerror.New("shared challenge caller binding is invalid")
	}
	return &boundClient{service: service, caller: caller}, nil
}

func (c *boundClient) Create(ctx context.Context, input challengecap.CreateRequest) (*challengecap.Presentation, error) {
	return c.service.create(ctx, c.caller, input)
}

func (c *boundClient) Complete(ctx context.Context, input challengecap.CompleteRequest) (*challengecap.Proof, error) {
	return c.service.complete(ctx, c.caller, input)
}

func (c *boundClient) Consume(ctx context.Context, input challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	return c.service.consume(ctx, c.caller, input)
}

func (s *serviceImpl) create(ctx context.Context, caller string, input challengecap.CreateRequest) (*challengecap.Presentation, error) {
	if err := challengecap.ValidateCreateRequest(input); err != nil {
		return nil, bizerr.WrapCode(err, sharedcode.CodeChallengeInvalid)
	}
	challengeNo, err := sharedutil.RandomToken(24)
	if err != nil {
		return nil, unavailable(err)
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, unavailable(err)
	}
	expiresAt := now.Add(challengeLifetime)
	presentation := &challengecap.Presentation{ChallengeNo: challengeNo, ProviderCode: s.config.Provider, ExpiresAt: expiresAt}
	data := modeldo.Challenge{TenantId: 0, ChallengeNo: challengeNo, CallerPluginId: caller, Purpose: input.Purpose,
		BindingDigest: digest(input.Binding), ProviderCode: s.config.Provider, Status: "pending", AttemptCount: 0,
		MaxAttempts: maximumAttempts, ExpiresAt: &expiresAt}
	if s.config.Provider == "local" {
		accessible := input.Kind == challengecap.PresentationAccessible
		answer, image, prompt, createErr := createLocalPresentation(accessible)
		if createErr != nil {
			return nil, unavailable(createErr)
		}
		salt := make([]byte, 32)
		if _, createErr = rand.Read(salt); createErr != nil {
			return nil, unavailable(createErr)
		}
		data.PresentationKind = string(input.Kind)
		data.AnswerSalt = hex.EncodeToString(salt)
		data.AnswerDigest = hex.EncodeToString(answerDigest(challengeNo, answer, salt))
		presentation.Kind = input.Kind
		if accessible {
			presentation.AccessiblePrompt = prompt
		} else {
			presentation.ImageMediaType = "image/png"
			presentation.ImageBytes = image
		}
	} else {
		data.PresentationKind = string(challengecap.PresentationTurnstile)
		presentation.Kind = challengecap.PresentationTurnstile
		presentation.SiteKey = s.config.TurnstileSiteKey
		presentation.ProviderAction = turnstileAction(caller, input.Purpose)
	}
	if _, err = s.database.Model(challengeTable).Ctx(ctx).Data(data).Insert(); err != nil {
		return nil, unavailable(err)
	}
	return presentation, nil
}

func (s *serviceImpl) complete(ctx context.Context, caller string, input challengecap.CompleteRequest) (*challengecap.Proof, error) {
	if err := challengecap.ValidateCompleteRequest(input); err != nil {
		return nil, bizerr.WrapCode(err, sharedcode.CodeChallengeInvalid)
	}
	if s.config.Provider == "turnstile" {
		if input.Answer != "" || input.ProviderToken == "" {
			return nil, bizerr.NewCode(sharedcode.CodeChallengeInvalid)
		}
		verified, err := s.verifyTurnstile(ctx, input.ProviderToken, turnstileAction(caller, input.Purpose))
		if err != nil {
			return nil, unavailable(err)
		}
		if !verified {
			if incrementErr := s.recordRejectedAttempt(ctx, caller, input); incrementErr != nil {
				return nil, incrementErr
			}
			return nil, bizerr.NewCode(sharedcode.CodeChallengeRejected)
		}
	} else if input.Answer == "" || input.ProviderToken != "" {
		return nil, bizerr.NewCode(sharedcode.CodeChallengeInvalid)
	}
	proofToken, err := sharedutil.RandomToken(32)
	if err != nil {
		return nil, unavailable(err)
	}
	var proofExpiresAt time.Time
	rejected := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		row, loadErr := s.loadPendingForUpdate(txCtx, caller, input.ChallengeNo, input.Purpose, input.Binding)
		if loadErr != nil {
			return loadErr
		}
		if s.config.Provider == "local" {
			salt, decodeErr := hex.DecodeString(row.AnswerSaltHex)
			if decodeErr != nil {
				return decodeErr
			}
			storedDigest, decodeErr := hex.DecodeString(row.AnswerDigestHex)
			if decodeErr != nil {
				return decodeErr
			}
			candidate := answerDigest(row.ChallengeNo, strings.TrimSpace(input.Answer), salt)
			if len(storedDigest) != len(candidate) || subtle.ConstantTimeCompare(storedDigest, candidate) != 1 {
				rejected = true
				return s.rejectLocked(txCtx, row)
			}
		}
		now, nowErr := databaseNow(txCtx, s.database)
		if nowErr != nil {
			return nowErr
		}
		proofExpiresAt = now.Add(proofLifetime)
		result, updateErr := s.database.Model(challengeTable).Ctx(txCtx).
			Where("tenant_id", 0).Where("challenge_no", row.ChallengeNo).Where("status", "pending").
			Data(modeldo.Challenge{Status: "verified", ProofDigest: sharedutil.Digest(proofToken),
				ProofExpiresAt: proofExpiresAt, VerifiedAt: now}).Update()
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.NewCode(sharedcode.CodeChallengeConflict)
		}
		return nil
	})
	if err != nil {
		return nil, publicError(err, sharedcode.CodeChallengeUnavailable)
	}
	if rejected {
		return nil, bizerr.NewCode(sharedcode.CodeChallengeRejected)
	}
	return &challengecap.Proof{Token: proofToken, ExpiresAt: proofExpiresAt}, nil
}

func (s *serviceImpl) consume(ctx context.Context, caller string, input challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	if err := challengecap.ValidateConsumeRequest(input); err != nil {
		return nil, bizerr.WrapCode(err, sharedcode.CodeChallengeInvalid)
	}
	var result challengecap.ConsumeResult
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row challengeRow
		if err := s.database.Model(challengeTable).Ctx(txCtx).Where("tenant_id", 0).
			Where("caller_plugin_id", caller).Where("purpose", input.Purpose).
			Where("binding_digest", digest(input.Binding)).Where("proof_digest", sharedutil.Digest(input.Token)).
			Where("status", "verified").Where("proof_expires_at > CURRENT_TIMESTAMP").
			LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row.ChallengeNo == "" {
			return bizerr.NewCode(sharedcode.CodeChallengeConflict)
		}
		now, err := databaseNow(txCtx, s.database)
		if err != nil {
			return err
		}
		update, err := s.database.Model(challengeTable).Ctx(txCtx).Where("tenant_id", 0).
			Where("challenge_no", row.ChallengeNo).Where("status", "verified").
			Data(modeldo.Challenge{Status: "consumed", ConsumedAt: now}).Update()
		if err != nil {
			return err
		}
		affected, err := update.RowsAffected()
		if err != nil || affected != 1 {
			return bizerr.NewCode(sharedcode.CodeChallengeConflict)
		}
		result = challengecap.ConsumeResult{ChallengeNo: row.ChallengeNo, ConsumedAt: now}
		return nil
	})
	if err != nil {
		return nil, publicError(err, sharedcode.CodeChallengeUnavailable)
	}
	return &result, nil
}

func (s *serviceImpl) loadPendingForUpdate(ctx context.Context, caller string, challengeNo string, purpose string, binding string) (*challengeRow, error) {
	var row challengeRow
	if err := s.database.Model(challengeTable).Ctx(ctx).
		Fields("challenge_no", "caller_plugin_id", "purpose", "binding_digest", "provider_code", "presentation_kind",
			"answer_salt AS answer_salt_hex", "answer_digest AS answer_digest_hex",
			"status", "attempt_count", "expires_at", "proof_expires_at").
		Where("tenant_id", 0).
		Where("challenge_no", challengeNo).Where("caller_plugin_id", caller).
		Where("purpose", purpose).Where("binding_digest", digest(binding)).Where("provider_code", s.config.Provider).
		Where("status", "pending").Where("expires_at > CURRENT_TIMESTAMP").LockUpdate().Scan(&row); err != nil {
		return nil, err
	}
	if row.ChallengeNo == "" {
		return nil, bizerr.NewCode(sharedcode.CodeChallengeConflict)
	}
	return &row, nil
}

func (s *serviceImpl) recordRejectedAttempt(ctx context.Context, caller string, input challengecap.CompleteRequest) error {
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		row, err := s.loadPendingForUpdate(txCtx, caller, input.ChallengeNo, input.Purpose, input.Binding)
		if err != nil {
			return err
		}
		return s.rejectLocked(txCtx, row)
	})
}

func (s *serviceImpl) rejectLocked(ctx context.Context, row *challengeRow) error {
	count := row.AttemptCount + 1
	status := "pending"
	if count >= maximumAttempts {
		status = "failed"
	}
	_, err := s.database.Model(challengeTable).Ctx(ctx).Where("tenant_id", 0).
		Where("challenge_no", row.ChallengeNo).Where("status", "pending").
		Data(modeldo.Challenge{AttemptCount: count, Status: status}).Update()
	if err != nil {
		return err
	}
	return nil
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, err
	}
	return value.Time(), nil
}

func answerDigest(challengeNo string, answer string, salt []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(challengeNo))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(salt)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strings.TrimSpace(answer)))
	return hash.Sum(nil)
}

func digest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func publicError(err error, fallback *bizerr.Code) error {
	for _, code := range []*bizerr.Code{sharedcode.CodeChallengeInvalid, sharedcode.CodeChallengeRejected, sharedcode.CodeChallengeConflict} {
		if bizerr.Is(err, code) {
			return err
		}
	}
	return bizerr.WrapCode(gerror.Wrap(err, "shared challenge operation failed"), fallback)
}

func unavailable(err error) error {
	return bizerr.WrapCode(gerror.Wrap(err, "shared challenge operation failed"), sharedcode.CodeChallengeUnavailable)
}

func (s *serviceImpl) CleanupExpired(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	_, err := s.database.Exec(ctx, `DELETE FROM plugin_shared_services_challenge WHERE id IN (
SELECT id FROM plugin_shared_services_challenge WHERE tenant_id=0 AND (
expires_at < CURRENT_TIMESTAMP - INTERVAL '24 hours' OR consumed_at < CURRENT_TIMESTAMP - INTERVAL '24 hours'
) ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED)`, limit)
	return err
}
