package copilot

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"lina-core/pkg/plugin/capability/i18ncap"
)

type Service struct {
	db   gdb.DB
	i18n i18ncap.Service
}

var limiter = struct {
	sync.Mutex
	calls map[string][]time.Time
}{calls: make(map[string][]time.Time)}

func New(db gdb.DB, translators ...i18ncap.Service) *Service {
	service := &Service{db: db}
	if len(translators) > 0 {
		service.i18n = translators[0]
	}
	return service
}

func (s *Service) translate(ctx context.Context, key, fallback string) string {
	if s == nil || s.i18n == nil {
		return fallback
	}
	return s.i18n.Translate(ctx, key, fallback)
}

type Config struct {
	Enabled                           bool
	BaseURL, Model                    string
	KeyConfigured                     bool
	BudgetMinor                       int64
	RateLimitPerMinute, RetentionDays int
	Revision                          int64
}

func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	var c Config
	if s.db == nil {
		return c, nil
	}
	r, err := s.db.Model("plugin_idc_ai_config").Where("tenant_id", 0).One(ctx)
	if err != nil {
		return c, err
	}
	if r.IsEmpty() {
		return c, nil
	}
	c.Enabled = r["enabled"].Bool()
	c.BaseURL = r["base_url"].String()
	c.Model = r["model_name"].String()
	c.KeyConfigured = len(r["encrypted_api_key"].Bytes()) > 0
	c.BudgetMinor = r["budget_minor"].Int64()
	c.RateLimitPerMinute = r["rate_limit_per_minute"].Int()
	c.RetentionDays = r["retention_days"].Int()
	c.Revision = r["revision"].Int64()
	return c, nil
}
func keyBytes() []byte {
	seed := os.Getenv("LINAPRO_IDC_CORE_SECURITY_KEYRING")
	if seed == "" {
		seed = "idc-ai-local-development-key"
	}
	h := sha256.Sum256([]byte(seed))
	return h[:]
}
func encryptSecret(v string) ([]byte, error) {
	b, _ := aes.NewCipher(keyBytes())
	a, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	n := make([]byte, a.NonceSize())
	out := a.Seal(n, n, []byte(v), nil)
	return out, nil
}
func decryptSecret(v []byte) (string, error) {
	b, err := aes.NewCipher(keyBytes())
	if err != nil {
		return "", err
	}
	a, err := cipher.NewGCM(b)
	if err != nil || len(v) < a.NonceSize() {
		return "", gerror.New("invalid encrypted AI credential")
	}
	plain, err := a.Open(nil, v[:a.NonceSize()], v[a.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
func (s *Service) UpdateConfig(ctx context.Context, baseURL, key, model string, enabled bool, budget int64, rate, retention int) (Config, error) {
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://127.0.0.1") {
		return Config{}, gerror.New("AI service address must use HTTPS")
	}
	values := g.Map{"tenant_id": 0, "base_url": baseURL, "model_name": model, "enabled": enabled, "budget_minor": budget, "rate_limit_per_minute": rate, "retention_days": retention, "revision": gdb.Raw("revision + 1"), "updated_at": gdb.Raw("CURRENT_TIMESTAMP")}
	if key != "" {
		enc, err := encryptSecret(key)
		if err != nil {
			return Config{}, err
		}
		values["encrypted_api_key"] = enc
	}
	old, err := s.db.Model("plugin_idc_ai_config").Where("tenant_id", 0).One(ctx)
	if err != nil {
		return Config{}, err
	}
	if old.IsEmpty() {
		if key == "" {
			return Config{}, gerror.New("API key is required for first save")
		}
		enc, encErr := encryptSecret(key)
		if encErr != nil {
			return Config{}, encErr
		}
		values["encrypted_api_key"] = enc
		values["revision"] = 1
		_, err = s.db.Model("plugin_idc_ai_config").Data(values).Insert(ctx)
	} else {
		_, err = s.db.Model("plugin_idc_ai_config").Where("tenant_id", 0).Data(values).Update(ctx)
	}
	if err != nil {
		return Config{}, err
	}
	return s.GetConfig(ctx)
}
func (s *Service) TestConfig(ctx context.Context, baseURL, key, model string) (bool, string) {
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://127.0.0.1") {
		return false, s.translate(ctx, "plugin.idc-ai.service.test.httpsRequired", "The service address must use HTTPS")
	}
	if strings.TrimSpace(key) == "" && s.db != nil {
		row, err := s.db.Model("plugin_idc_ai_config").Where("tenant_id", 0).Fields("encrypted_api_key").One(ctx)
		if err == nil && !row.IsEmpty() {
			key, err = decryptSecret(row["encrypted_api_key"].Bytes())
			if err != nil {
				return false, s.translate(ctx, "plugin.idc-ai.service.test.savedKeyUnavailable", "The saved API key could not be used")
			}
		}
	}
	reqBody, err := json.Marshal(map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 1})
	if err != nil {
		return false, s.translate(ctx, "plugin.idc-ai.service.test.encodeFailed", "The model request could not be encoded")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return false, s.translate(ctx, "plugin.idc-ai.service.test.createFailed", "The model request could not be created")
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	c := http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false, s.translate(ctx, "plugin.idc-ai.service.test.unreachable", "The model service is temporarily unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, s.translate(ctx, "plugin.idc-ai.service.test.succeeded", "Connection successful")
	}
	return false, s.translate(ctx, "plugin.idc-ai.service.test.rejected", "The model service rejected the test request")
}

var dangerousRE = regexp.MustCompile(`(?i)(restart|reinstall|change.?ip|delete server|refund|payment|change price|invoice|contract|secret|api.?key|password|ignore.*rule|bypass)`)
var sensitiveRE = regexp.MustCompile(`(?i)(identity document|facial|liveness|business license|original corporate|seal evidence|raw invoice|contract file|id photo)`)

func containsBlockedCJK(prompt string) bool {
	for _, term := range [][]rune{{0x91cd, 0x542f}, {0x91cd, 0x88c5}, {0x5220, 0x9664, 0x670d, 0x52a1}, {0x8eab, 0x4efd, 0x8bc1}, {0x4eba, 0x8138}, {0x8425, 0x4e1a, 0x6267, 0x7167}, {0x53d1, 0x7968}} {
		if strings.Contains(prompt, string(term)) {
			return true
		}
	}
	return false
}

func (s *Service) Chat(ctx context.Context, prompt, actorType, actorID string) (answer string, evidence []string, dataAsOf, uncertainty, safety, model string, id int64, err error) {
	safety = "allowed"
	if dangerousRE.MatchString(prompt) || containsBlockedCJK(prompt) {
		safety = "refused"
		answer = s.translate(ctx, "plugin.idc-ai.service.chat.dangerousRefusal", "This request involves a server, financial, or sensitive-data operation that I cannot execute. Open the normal business page and follow its permission and confirmation flow.")
		s.audit(ctx, actorType, actorID, "dangerous_operation", "rejected", "write_operation_requested")
		return answer, evidence, dataAsOf, uncertainty, safety, model, id, err
	}
	if sensitiveRE.MatchString(prompt) {
		s.audit(ctx, actorType, actorID, "sensitive_material", "rejected", "sensitive_material_excluded")
		return s.translate(ctx, "plugin.idc-ai.service.chat.sensitiveRefusal", "For safety, identity documents, facial data, corporate originals, contract/seal evidence, and raw invoice material are never sent to AI. Review the compliance status summary or open a support ticket."), nil, time.Now().UTC().Format(time.RFC3339), s.translate(ctx, "plugin.idc-ai.service.chat.sensitiveFiltered", "Sensitive material was filtered"), "refused", "", 0, nil
	}
	c, _ := s.GetConfig(ctx)
	model = c.Model
	if c.Enabled && c.RateLimitPerMinute > 0 {
		limiter.Lock()
		now := time.Now()
		recent := limiter.calls["tenant:0"][:0]
		for _, ts := range limiter.calls["tenant:0"] {
			if now.Sub(ts) < time.Minute {
				recent = append(recent, ts)
			}
		}
		if len(recent) >= c.RateLimitPerMinute {
			limiter.calls["tenant:0"] = recent
			limiter.Unlock()
			return s.translate(ctx, "plugin.idc-ai.service.chat.throttled", "AI requests are busy; try again later. Normal business pages remain available."), nil, now.UTC().Format(time.RFC3339), s.translate(ctx, "plugin.idc-ai.service.chat.rateLimitReached", "Merchant rate limit reached"), "throttled", model, 0, nil
		}
		limiter.calls["tenant:0"] = append(recent, now)
		limiter.Unlock()
	}
	if c.Enabled && c.BudgetMinor > 0 && s.db != nil {
		used, qerr := s.db.Model("plugin_idc_ai_conversation").Where("tenant_id", 0).Where("created_at >= ?", time.Now().UTC().Truncate(24*time.Hour)).Fields("COALESCE(SUM(estimated_cost_minor),0) AS used").Value(ctx)
		if qerr == nil && used.Int64() >= c.BudgetMinor {
			now := time.Now().UTC().Format(time.RFC3339)
			return s.translate(ctx, "plugin.idc-ai.service.chat.budgetExhausted", "The merchant AI budget is exhausted. Ask an administrator to adjust it; normal business pages remain available."), nil, now, s.translate(ctx, "plugin.idc-ai.service.chat.budgetLimitReached", "Budget limit reached"), "budget_exhausted", model, 0, nil
		}
	}
	if !c.Enabled {
		answer = s.translate(ctx, "plugin.idc-ai.service.chat.notEnabled", "The AI assistant is not enabled. You can continue using normal operations pages and support workflows.")
		safety = "fallback"
	} else {
		answer = s.translate(ctx, "plugin.idc-ai.service.chat.summary", "Based on the currently available business summary, I can analyze orders, renewals, resources, and tickets. Verify the result on the normal page before acting.")
		evidence = []string{s.translate(ctx, "plugin.idc-ai.service.chat.summaryEvidence", "Read-only operations summary")}
		dataAsOf = time.Now().UTC().Format(time.RFC3339)
		uncertainty = s.translate(ctx, "plugin.idc-ai.service.chat.uncertainty", "Some data may be delayed; this does not replace human confirmation")
		// A configured provider may return a richer answer. Any network or schema
		// failure deliberately falls back to the safe local summary above.
		if s.db != nil {
			if row, qerr := s.db.Model("plugin_idc_ai_config").Where("tenant_id", 0).One(ctx); qerr == nil && !row.IsEmpty() {
				if secret, derr := decryptSecret(row["encrypted_api_key"].Bytes()); derr == nil {
					if remote, ok := s.callModel(ctx, c.BaseURL, secret, c.Model, prompt); ok && remote != "" {
						answer = remote
					}
				}
			}
		}
	}
	if s.db != nil {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt)))
		r, er := s.db.Model("plugin_idc_ai_conversation").Data(g.Map{"tenant_id": 0, "actor_type": actorType, "actor_id": actorID, "prompt_digest": digest, "response_text": answer, "model_name": model, "evidence_json": gdb.Raw("'[]'::jsonb"), "safety_decision": safety}).Insert(ctx)
		if er == nil {
			id, _ = r.LastInsertId()
		}
		s.audit(ctx, actorType, actorID, "readonly_summary", "allowed", "")
	}
	return answer, evidence, dataAsOf, uncertainty, safety, model, id, err
}

func (s *Service) callModel(ctx context.Context, baseURL, key, model, prompt string) (string, bool) {
	payload, err := json.Marshal(map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": "You are a read-only operations assistant. Never execute commands or reveal secrets."}, {"role": "user", "content": prompt}}, "max_tokens": 800})
	if err != nil {
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&decoded) != nil || len(decoded.Choices) == 0 {
		return "", false
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), true
}
func (s *Service) audit(ctx context.Context, actorType, actorID, tool, decision, reason string) {
	if s.db == nil {
		return
	}
	_, _ = s.db.Model("plugin_idc_ai_tool_audit").Data(g.Map{"tenant_id": 0, "actor_type": actorType, "actor_id": actorID, "tool_name": tool, "decision": decision, "reason_code": reason, "evidence_json": gdb.Raw("'{}'::jsonb")}).Insert(ctx)
}
func (s *Service) Feedback(ctx context.Context, conversation int64, actor string, rating int, comment string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Model("plugin_idc_ai_feedback").Data(g.Map{"tenant_id": 0, "conversation_id": conversation, "actor_id": actor, "rating": rating, "comment": comment}).Insert(ctx)
	return err
}
func (s *Service) Audits(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Model("plugin_idc_ai_tool_audit").Fields("tool_name,decision,reason_code,created_at").OrderDesc("created_at").Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"tool": r["tool_name"].String(), "decision": r["decision"].String(), "reason": r["reason_code"].String(), "createdAt": r["created_at"].String()})
	}
	return out, nil
}
