// Package reconciliation owns bounded daily provider reconciliation and the
// staff-facing two-person supplement review workflow.
package reconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
)

const (
	DefaultLimit = 500
	maxLimit     = 1000
)

type SourceRecord struct {
	RecordType, LocalSubjectNo, ProviderEventReference, ExternalReference string
	CurrencyCode, StatusCode, EvidenceDigest                              string
	AmountMinor                                                           int64
	ObservedAt                                                            time.Time
}

type SourceLoader interface {
	Load(context.Context, string, time.Time, int) ([]SourceRecord, string, error)
}

type Service struct {
	database gdb.DB
	channels *channelconfig.Service
	attempts *paymentattempt.Store
	loader   SourceLoader
}

type BatchResult struct {
	BatchNo, ChannelNo, CapabilityCode, StatusCode, FailureCode string
	BusinessDate                                                time.Time
	TotalCount, DifferenceCount, PaymentCount, RefundCount      int
	ResultUnknown                                               bool
}

type ChannelHealth struct {
	ChannelNo, DisplayName, ProviderCode, CapabilityCode, HealthCode string
	LatestBatchNo, FailureCode                                       string
	LatestBusinessDate                                               *time.Time
	UnresolvedCount, ConsecutiveFailureCount                         int
	ResultUnknown                                                    bool
}

type BatchPage struct {
	Items []BatchResult
	Total int
}

// HealthPage contains one channel-health page and its exact total.
type HealthPage struct {
	Items []ChannelHealth
	Total int
}

// DifferencePage contains one difference page and its exact batch-scoped total.
type DifferencePage struct {
	Items []Difference
	Total int
}

type Difference struct {
	LineNo, BatchNo, RecordType, AttemptNo, RefundAttemptNo string
	ExternalReference, DifferenceCode, CurrencyCode         string
	LocalStatusCode, ChannelStatusCode, ResolutionCode      string
	SupplementRequestNo, SupplementStatusCode               string
	LocalAmountMinor, ChannelAmountMinor                    int64
	SupplementSubmitterUserID                               int
	Version, SupplementVersion                              int64
}

type Supplement struct {
	RequestNo, LineNo, AttemptNo, ChannelNo, CurrencyCode string
	ExternalReference, StatusCode, RejectionReasonCode    string
	AmountMinor                                           int64
	SubmitterUserID, ReviewerUserID                       int
	Version                                               int64
}

func New(database gdb.DB, channels *channelconfig.Service, attempts *paymentattempt.Store, loader SourceLoader) (*Service, error) {
	if database == nil || channels == nil || attempts == nil || loader == nil {
		return nil, gerror.New("reconciliation dependencies are required")
	}
	return &Service{database: database, channels: channels, attempts: attempts, loader: loader}, nil
}

func (s *Service) RunAll(ctx context.Context, businessDate time.Time, timezone string, channelLimit int) ([]BatchResult, error) {
	if channelLimit <= 0 || channelLimit > 100 {
		return nil, gerror.New("invalid reconciliation channel limit")
	}
	rows, err := s.database.GetAll(ctx, `SELECT channel_no FROM plugin_idc_payment_channel WHERE tenant_id=0 AND status_code='enabled' AND deleted_at IS NULL ORDER BY channel_no LIMIT ?`, channelLimit)
	if err != nil {
		return nil, err
	}
	result := make([]BatchResult, 0, len(rows))
	for _, row := range rows {
		batch, runErr := s.RunChannel(ctx, row["channel_no"].String(), businessDate, timezone, DefaultLimit)
		if runErr != nil {
			return result, runErr
		}
		result = append(result, *batch)
	}
	return result, nil
}

func (s *Service) RunChannel(ctx context.Context, channelNo string, businessDate time.Time, timezone string, limit int) (*BatchResult, error) {
	if strings.TrimSpace(channelNo) == "" || limit <= 0 || limit > maxLimit {
		return nil, gerror.New("invalid reconciliation request")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, gerror.New("invalid merchant timezone")
	}
	day := businessDate.In(location)
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	channel, err := s.database.GetOne(ctx, `SELECT channel_no,capability_snapshot FROM plugin_idc_payment_channel WHERE tenant_id=0 AND channel_no=? AND status_code='enabled' AND deleted_at IS NULL`, channelNo)
	if err != nil {
		return nil, err
	}
	if channel.IsEmpty() {
		return nil, gerror.New("payment channel is unavailable")
	}
	capabilities := map[string]bool{}
	if json.Unmarshal(channel["capability_snapshot"].Bytes(), &capabilities) != nil {
		return nil, gerror.New("payment channel capability is invalid")
	}
	capability := "unavailable"
	if capabilities[string(paymentcap.OperationStatement)] {
		capability = "statement"
	} else if capabilities[string(paymentcap.OperationQuery)] {
		capability = "authoritative_query"
	}
	scope := channelNo + ":" + day.Format("2006-01-02")
	batchNo := stableNo("prb", scope)
	if capability == "unavailable" {
		return s.recordUnavailable(ctx, batchNo, scope, channelNo, day, timezone)
	}
	records, sourceDigest, err := s.loader.Load(ctx, channelNo, day, limit)
	if err != nil {
		return s.recordFailure(ctx, batchNo, scope, channelNo, day, timezone, capability, "SOURCE_UNAVAILABLE")
	}
	if len(records) > limit || len(sourceDigest) != 64 {
		return s.recordFailure(ctx, batchNo, scope, channelNo, day, timezone, capability, "SOURCE_INVALID")
	}
	startUTC, endUTC := day.UTC(), day.AddDate(0, 0, 1).UTC()
	localPayments, err := s.database.GetAll(ctx, `SELECT attempt_no,customer_ref_digest,currency_code,(amount_minor-balance_locked_minor) AS amount_minor,external_trade_reference,status_code FROM plugin_idc_payment_attempt WHERE channel_no=? AND created_at>=? AND created_at<? ORDER BY id LIMIT ?`, channelNo, startUTC, endUTC, limit)
	if err != nil {
		return nil, err
	}
	localRefunds, err := s.database.GetAll(ctx, `SELECT refund_attempt_no,payment_attempt_no,currency_code,amount_minor,external_refund_reference,status_code FROM plugin_idc_payment_refund_attempt WHERE channel_no=? AND requested_at>=? AND requested_at<? ORDER BY id LIMIT ?`, channelNo, startUTC, endUTC, limit)
	if err != nil {
		return nil, err
	}
	if len(localPayments) == limit || len(localRefunds) == limit {
		return s.recordFailure(ctx, batchNo, scope, channelNo, day, timezone, capability, "LOCAL_LIMIT_REACHED")
	}

	result := &BatchResult{BatchNo: batchNo, ChannelNo: channelNo, CapabilityCode: capability, BusinessDate: day}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(`INSERT INTO plugin_idc_payment_reconciliation_batch(batch_no,channel_no,business_date,source_digest,status_code,daily_scope_key,merchant_timezone,source_type_code,capability_code) VALUES(?,?,?,?,'processing',?,?,?,?) ON CONFLICT(daily_scope_key) WHERE daily_scope_key<>'' DO NOTHING`, batchNo, channelNo, day.Format("2006-01-02"), sourceDigest, scope, timezone, capability, capability); txErr != nil {
			return txErr
		}
		existing, txErr := tx.GetOne(`SELECT status_code,total_count,difference_count,payment_count,refund_count,result_unknown,failure_code FROM plugin_idc_payment_reconciliation_batch WHERE daily_scope_key=? FOR UPDATE`, scope)
		if txErr != nil || existing.IsEmpty() {
			return txErr
		}
		if existing["status_code"].String() != "processing" {
			fillBatchResult(result, existing)
			return nil
		}
		lines, paymentCount, refundCount, txErr := buildLines(batchNo, records, localPayments, localRefunds)
		if txErr != nil {
			return txErr
		}
		if txErr = s.persistSources(ctx, batchNo, records, localPayments); txErr != nil {
			return txErr
		}
		differenceCount, txErr := s.persistLines(ctx, batchNo, lines)
		if txErr != nil {
			return txErr
		}
		status := "matched"
		if differenceCount > 0 {
			status = "differences"
		}
		if _, txErr = tx.Exec(`UPDATE plugin_idc_payment_reconciliation_batch SET status_code=?,total_count=?,difference_count=?,payment_count=?,refund_count=?,finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE batch_no=? AND status_code='processing'`, status, len(lines), differenceCount, paymentCount, refundCount, batchNo); txErr != nil {
			return txErr
		}
		health := "healthy"
		if differenceCount > 0 {
			health = "differences"
		}
		_, txErr = tx.Exec(`INSERT INTO plugin_idc_payment_reconciliation_channel_health(channel_no,capability_code,health_code,latest_batch_no,latest_business_date,unresolved_count) VALUES(?,?,?,?,?,?) ON CONFLICT(channel_no) DO UPDATE SET capability_code=EXCLUDED.capability_code,health_code=EXCLUDED.health_code,latest_batch_no=EXCLUDED.latest_batch_no,latest_business_date=EXCLUDED.latest_business_date,unresolved_count=EXCLUDED.unresolved_count,consecutive_failure_count=0,result_unknown=FALSE,failure_code='',version=plugin_idc_payment_reconciliation_channel_health.version+1,updated_at=CURRENT_TIMESTAMP`, channelNo, capability, health, batchNo, day.Format("2006-01-02"), differenceCount)
		result.StatusCode, result.TotalCount, result.DifferenceCount = status, len(lines), differenceCount
		result.PaymentCount, result.RefundCount = paymentCount, refundCount
		return txErr
	})
	return result, err
}

type builtLine struct {
	LineNo, RecordType, AttemptNo, RefundAttemptNo, ExternalReference, DifferenceCode string
	CurrencyCode, EvidenceDigest, CustomerDigest, LocalStatus, ChannelStatus          string
	LocalAmountMinor, ChannelAmountMinor                                              int64
}

func buildLines(batchNo string, records []SourceRecord, payments, refunds gdb.Result) ([]builtLine, int, int, error) {
	type local struct {
		subject, customer, currency, external, status string
		amount                                        int64
		seen                                          bool
	}
	locals := map[string]*local{}
	for _, row := range payments {
		value := &local{subject: row["attempt_no"].String(), customer: row["customer_ref_digest"].String(), currency: row["currency_code"].String(), external: row["external_trade_reference"].String(), status: row["status_code"].String(), amount: row["amount_minor"].Int64()}
		locals["payment:"+value.subject] = value
	}
	for _, row := range refunds {
		value := &local{subject: row["refund_attempt_no"].String(), currency: row["currency_code"].String(), external: row["external_refund_reference"].String(), status: row["status_code"].String(), amount: row["amount_minor"].Int64()}
		locals["refund:"+value.subject] = value
	}
	lines := make([]builtLine, 0, len(records)+len(locals))
	externalSeen := map[string]int{}
	paymentCount, refundCount := 0, 0
	for _, record := range records {
		if err := validateSource(record); err != nil {
			return nil, 0, 0, err
		}
		if record.RecordType == "payment" {
			paymentCount++
		} else {
			refundCount++
		}
		key := record.RecordType + ":" + record.LocalSubjectNo
		localValue := locals[key]
		difference, localAmount, localStatus, customer := "local_missing", int64(0), "", strings.Repeat("0", 64)
		if localValue != nil {
			localValue.seen = true
			localAmount, localStatus, customer = localValue.amount, localValue.status, localValue.customer
			difference = "none"
			if localValue.currency != record.CurrencyCode || localValue.amount != record.AmountMinor {
				difference = "amount_mismatch"
			} else if !statusesMatch(record.RecordType, localValue.status, record.StatusCode) {
				difference = "status_mismatch"
			} else if localValue.external != "" && localValue.external != record.ExternalReference {
				difference = "duplicate_transaction"
			}
		}
		externalKey := record.RecordType + ":" + record.ExternalReference
		externalSeen[externalKey]++
		if externalSeen[externalKey] > 1 {
			difference = "duplicate_transaction"
		}
		lines = append(lines, builtLine{LineNo: stableNo("prl", batchNo, record.RecordType, record.ProviderEventReference), RecordType: record.RecordType, AttemptNo: choose(record.RecordType == "payment", record.LocalSubjectNo, ""), RefundAttemptNo: choose(record.RecordType == "refund", record.LocalSubjectNo, ""), ExternalReference: record.ExternalReference, DifferenceCode: difference, CurrencyCode: record.CurrencyCode, EvidenceDigest: record.EvidenceDigest, CustomerDigest: customer, LocalStatus: localStatus, ChannelStatus: record.StatusCode, LocalAmountMinor: localAmount, ChannelAmountMinor: record.AmountMinor})
	}
	keys := make([]string, 0, len(locals))
	for key := range locals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := locals[key]
		if value.seen {
			continue
		}
		recordType := strings.SplitN(key, ":", 2)[0]
		lines = append(lines, builtLine{LineNo: stableNo("prl", batchNo, recordType, value.subject), RecordType: recordType, AttemptNo: choose(recordType == "payment", value.subject, ""), RefundAttemptNo: choose(recordType == "refund", value.subject, ""), ExternalReference: value.external, DifferenceCode: "provider_missing", CurrencyCode: value.currency, EvidenceDigest: stableDigest(batchNo, key, "provider_missing"), CustomerDigest: zeroDigest(value.customer), LocalStatus: value.status, LocalAmountMinor: value.amount})
	}
	return lines, paymentCount, refundCount, nil
}

func validateSource(value SourceRecord) error {
	if (value.RecordType != "payment" && value.RecordType != "refund") || value.LocalSubjectNo == "" || value.ProviderEventReference == "" || value.ExternalReference == "" || len(value.CurrencyCode) != 3 || value.AmountMinor <= 0 || value.StatusCode == "" || len(value.EvidenceDigest) != 64 || value.ObservedAt.IsZero() {
		return gerror.New("invalid reconciliation source record")
	}
	return nil
}

func statusesMatch(recordType, local, provider string) bool {
	if provider == "succeeded" {
		return local == "succeeded"
	}
	if provider == "refunded" && recordType == "refund" {
		return local == "succeeded" || local == "manual_confirmed"
	}
	return provider == local
}

func (s *Service) recordUnavailable(ctx context.Context, batchNo, scope, channelNo string, day time.Time, timezone string) (*BatchResult, error) {
	return s.recordTerminal(ctx, batchNo, scope, channelNo, day, timezone, "unavailable", "failed", "CAPABILITY_UNAVAILABLE", true)
}
func (s *Service) recordFailure(ctx context.Context, batchNo, scope, channelNo string, day time.Time, timezone, capability, code string) (*BatchResult, error) {
	return s.recordTerminal(ctx, batchNo, scope, channelNo, day, timezone, capability, "failed", code, true)
}
func (s *Service) recordTerminal(ctx context.Context, batchNo, scope, channelNo string, day time.Time, timezone, capability, status, failure string, unknown bool) (*BatchResult, error) {
	source := stableDigest(scope, capability, failure)
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_batch(batch_no,channel_no,business_date,source_digest,status_code,daily_scope_key,merchant_timezone,source_type_code,capability_code,result_unknown,failure_code,finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP) ON CONFLICT(daily_scope_key) WHERE daily_scope_key<>'' DO NOTHING`, batchNo, channelNo, day.Format("2006-01-02"), source, status, scope, timezone, capability, capability, unknown, failure)
	if err != nil {
		return nil, err
	}
	health := "failed"
	if capability == "unavailable" {
		health = "unavailable"
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_channel_health(channel_no,capability_code,health_code,latest_batch_no,latest_business_date,result_unknown,failure_code,consecutive_failure_count) VALUES(?,?,?,?,?,TRUE,?,1) ON CONFLICT(channel_no) DO UPDATE SET capability_code=EXCLUDED.capability_code,health_code=EXCLUDED.health_code,latest_batch_no=EXCLUDED.latest_batch_no,latest_business_date=EXCLUDED.latest_business_date,result_unknown=TRUE,failure_code=EXCLUDED.failure_code,consecutive_failure_count=plugin_idc_payment_reconciliation_channel_health.consecutive_failure_count+1,version=plugin_idc_payment_reconciliation_channel_health.version+1,updated_at=CURRENT_TIMESTAMP`, channelNo, capability, health, batchNo, day.Format("2006-01-02"), failure)
	return &BatchResult{BatchNo: batchNo, ChannelNo: channelNo, CapabilityCode: capability, StatusCode: status, FailureCode: failure, BusinessDate: day, ResultUnknown: unknown}, err
}

func (s *Service) SubmitSupplement(ctx context.Context, lineNo string, actorID int, expectedVersion int64) (*Supplement, error) {
	if lineNo == "" || actorID <= 0 || expectedVersion <= 0 {
		return nil, gerror.New("invalid supplement submission")
	}
	var result *Supplement
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := tx.GetOne(`SELECT l.line_no,l.attempt_no,l.external_reference,l.difference_code,l.channel_amount_minor,l.currency_code,l.channel_status_code,l.evidence_digest,l.customer_ref_digest,l.version,b.channel_no,a.customer_ref_digest AS actual_customer,a.currency_code AS actual_currency,(a.amount_minor-a.balance_locked_minor) AS actual_amount,a.status_code,a.external_trade_reference FROM plugin_idc_payment_reconciliation_line l JOIN plugin_idc_payment_reconciliation_batch b ON b.batch_no=l.batch_no JOIN plugin_idc_payment_reconciliation_source src ON src.batch_no=l.batch_no AND src.record_type_code=l.record_type_code AND src.local_subject_no=l.attempt_no AND src.external_reference=l.external_reference AND src.evidence_digest=l.evidence_digest JOIN plugin_idc_payment_attempt a ON a.attempt_no=l.attempt_no WHERE l.line_no=? FOR UPDATE`, lineNo)
		if err != nil {
			return err
		}
		if row.IsEmpty() {
			return gerror.New("reconciliation difference is not visible")
		}
		if row["version"].Int64() != expectedVersion {
			return gerror.New("reconciliation difference version changed")
		}
		if row["difference_code"].String() != "local_missing" || row["channel_status_code"].String() != "succeeded" || row["status_code"].String() == "succeeded" || row["external_reference"].String() == "" || row["customer_ref_digest"].String() != row["actual_customer"].String() || row["currency_code"].String() != row["actual_currency"].String() || row["channel_amount_minor"].Int64() != row["actual_amount"].Int64() || (row["external_trade_reference"].String() != "" && row["external_trade_reference"].String() != row["external_reference"].String()) {
			return gerror.New("reconciliation difference is not eligible for supplementation")
		}
		content := stableDigest(lineNo, row["attempt_no"].String(), row["channel_no"].String(), row["actual_customer"].String(), row["actual_currency"].String(), strconv.FormatInt(row["actual_amount"].Int64(), 10), row["external_reference"].String(), row["evidence_digest"].String())
		requestNo := stableNo("psr", lineNo)
		_, err = tx.Exec(`INSERT INTO plugin_idc_payment_reconciliation_supplement(request_no,line_no,attempt_no,channel_no,customer_ref_digest,currency_code,amount_minor,external_reference,evidence_digest,content_digest,submitter_user_id) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(line_no) DO NOTHING`, requestNo, lineNo, row["attempt_no"].String(), row["channel_no"].String(), row["actual_customer"].String(), row["actual_currency"].String(), row["actual_amount"].Int64(), row["external_reference"].String(), row["evidence_digest"].String(), content, actorID)
		if err != nil {
			return err
		}
		created, err := tx.GetOne(`SELECT request_no,line_no,attempt_no,channel_no,currency_code,amount_minor,external_reference,status_code,submitter_user_id,reviewer_user_id,rejection_reason_code,version FROM plugin_idc_payment_reconciliation_supplement WHERE line_no=?`, lineNo)
		if err != nil {
			return err
		}
		result = supplementFromRecord(created)
		return nil
	})
	return result, err
}

func (s *Service) ReviewSupplement(ctx context.Context, requestNo string, actorID int, approve bool, rejectionReason string, expectedVersion int64) (*Supplement, error) {
	if requestNo == "" || actorID <= 0 || expectedVersion <= 0 {
		return nil, gerror.New("invalid supplement review")
	}
	if approve && rejectionReason != "" {
		return nil, gerror.New("approved supplement cannot have rejection reason")
	}
	if !approve && !validRejection(rejectionReason) {
		return nil, gerror.New("invalid supplement rejection reason")
	}
	if approve {
		if err := s.attempts.ApplyReconciliationSupplement(ctx, paymentattempt.ReconciliationSupplementInput{RequestNo: requestNo, ReviewerUserID: actorID, ExpectedVersion: expectedVersion}); err != nil {
			return nil, err
		}
	} else {
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_reconciliation_supplement SET status_code='rejected',reviewer_user_id=?,rejection_reason_code=?,reviewed_at=CURRENT_TIMESTAMP,version=version+1 WHERE request_no=? AND status_code='pending_review' AND submitter_user_id<>? AND version=?`, actorID, rejectionReason, requestNo, actorID, expectedVersion)
		if err != nil {
			return nil, err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return nil, gerror.New("supplement review was rejected or changed")
		}
	}
	row, err := s.database.GetOne(ctx, `SELECT request_no,line_no,attempt_no,channel_no,currency_code,amount_minor,external_reference,status_code,submitter_user_id,reviewer_user_id,rejection_reason_code,version FROM plugin_idc_payment_reconciliation_supplement WHERE request_no=?`, requestNo)
	if err != nil {
		return nil, err
	}
	return supplementFromRecord(row), nil
}

func validRejection(value string) bool {
	switch value {
	case "evidence_insufficient", "ownership_mismatch", "amount_or_currency_mismatch", "provider_reference_conflict", "already_confirmed":
		return true
	}
	return false
}
func supplementFromRecord(row gdb.Record) *Supplement {
	if row.IsEmpty() {
		return nil
	}
	return &Supplement{RequestNo: row["request_no"].String(), LineNo: row["line_no"].String(), AttemptNo: row["attempt_no"].String(), ChannelNo: row["channel_no"].String(), CurrencyCode: row["currency_code"].String(), AmountMinor: row["amount_minor"].Int64(), ExternalReference: row["external_reference"].String(), StatusCode: row["status_code"].String(), SubmitterUserID: row["submitter_user_id"].Int(), ReviewerUserID: row["reviewer_user_id"].Int(), RejectionReasonCode: row["rejection_reason_code"].String(), Version: row["version"].Int64()}
}
func zeroDigest(value string) string {
	if len(value) == 64 {
		return value
	}
	return strings.Repeat("0", 64)
}
func choose(condition bool, first, second string) string {
	if condition {
		return first
	}
	return second
}
func fillBatchResult(result *BatchResult, row gdb.Record) {
	result.StatusCode = row["status_code"].String()
	result.TotalCount = row["total_count"].Int()
	result.DifferenceCount = row["difference_count"].Int()
	result.PaymentCount = row["payment_count"].Int()
	result.RefundCount = row["refund_count"].Int()
	result.ResultUnknown = row["result_unknown"].Bool()
	result.FailureCode = row["failure_code"].String()
}
func stableDigest(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}
func stableNo(prefix string, values ...string) string {
	return prefix + "_" + stableDigest(values...)[:32]
}
