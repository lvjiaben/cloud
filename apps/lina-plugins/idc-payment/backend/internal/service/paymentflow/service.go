package paymentflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
)

type Config struct{ PublicOrigin string }
type Service struct {
	attempts *paymentattempt.Store
	channels *channelconfig.Service
	origin   string
	ready    bool
}
type CreateInput struct {
	PaymentContextNo, ChannelNo string
	Mode                        paymentcap.Mode
	BalanceAmountMinor          int64
	ClientIP, OpenID            string
}
type CreditCreateInput struct {
	StatementNo, ChannelNo string
	AmountMinor            int64
	Mode                   paymentcap.Mode
	ClientIP, OpenID       string
}
type RechargeCreateInput struct {
	ChannelNo, CurrencyCode string
	AmountMinor             int64
	Mode                    paymentcap.Mode
	ClientIP, OpenID        string
}
type Created struct {
	AttemptNo  string
	StatusCode string
	Redirect   paymentcap.Redirect
	ExpiresAt  time.Time
}

func New(attempts *paymentattempt.Store, channels *channelconfig.Service, config Config) (*Service, error) {
	if attempts == nil || channels == nil {
		return nil, gerror.New("payment flow dependencies are required")
	}
	origin, err := url.Parse(strings.TrimRight(config.PublicOrigin, "/"))
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, gerror.New("payment public origin must use HTTPS")
	}
	return &Service{attempts: attempts, channels: channels, origin: origin.String(), ready: !strings.HasSuffix(strings.ToLower(origin.Hostname()), ".invalid")}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*Created, error) {
	if !s.ready {
		return nil, gerror.New("payment public origin is not configured")
	}
	attemptNo, err := randomNo("PAT_")
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(input)
	if err != nil {
		return nil, err
	}
	prepared, err := s.attempts.PrepareAndCreate(ctx, paymentattempt.PrepareInput{AttemptNo: attemptNo, PaymentContextNo: input.PaymentContextNo, ChannelNo: input.ChannelNo, ModeCode: string(input.Mode), RequestDigest: digest, BalanceAmountMinor: input.BalanceAmountMinor})
	if err != nil {
		return nil, err
	}
	return s.submit(ctx, attemptNo, input.ChannelNo, prepared.InvoiceNo, input.Mode, prepared.CurrencyCode, prepared.ChannelAmountMinor, "IDC server order "+prepared.OrderNo, input.ClientIP, input.OpenID, prepared.ExpiresAt)
}

func (s *Service) CreateCreditRepayment(ctx context.Context, input CreditCreateInput) (*Created, error) {
	if !s.ready {
		return nil, gerror.New("payment public origin is not configured")
	}
	attemptNo, err := randomNo("PAT_")
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(input)
	if err != nil {
		return nil, err
	}
	prepared, err := s.attempts.PrepareCreditAndCreate(ctx, paymentattempt.CreditPrepareInput{AttemptNo: attemptNo, StatementNo: input.StatementNo, ChannelNo: input.ChannelNo, ModeCode: string(input.Mode), RequestDigest: digest, AmountMinor: input.AmountMinor})
	if err != nil {
		return nil, err
	}
	return s.submit(ctx, attemptNo, input.ChannelNo, prepared.StatementNo, input.Mode, prepared.CurrencyCode, prepared.AmountMinor, "IDC credit statement "+prepared.StatementNo, input.ClientIP, input.OpenID, prepared.ExpiresAt)
}

func (s *Service) CreateBalanceRecharge(ctx context.Context, input RechargeCreateInput) (*Created, error) {
	if !s.ready {
		return nil, gerror.New("payment public origin is not configured")
	}
	attemptNo, err := randomNo("PAT_")
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(input)
	if err != nil {
		return nil, err
	}
	prepared, err := s.attempts.PrepareRechargeAndCreate(ctx, paymentattempt.RechargePrepareInput{AttemptNo: attemptNo, ChannelNo: input.ChannelNo, ModeCode: string(input.Mode), RequestDigest: digest, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor})
	if err != nil {
		return nil, err
	}
	return s.submit(ctx, attemptNo, input.ChannelNo, prepared.RechargeContextNo, input.Mode, prepared.CurrencyCode, prepared.AmountMinor, "IDC account balance recharge", input.ClientIP, input.OpenID, prepared.ExpiresAt)
}

func (s *Service) submit(ctx context.Context, attemptNo, channelNo, billingReference string, mode paymentcap.Mode, currency string, amountMinor int64, subject, clientIP, openID string, expiresAt time.Time) (*Created, error) {
	var redirect *paymentcap.Redirect
	err := s.channels.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		var createErr error
		redirect, createErr = adapter.Create(ctx, credentials, paymentcap.CreateRequest{AttemptNo: attemptNo, InvoiceNo: billingReference, Mode: mode, Currency: currency, AmountMinor: amountMinor, Subject: subject, NotifyURL: s.origin + "/api/v1/callbacks/payments/" + url.PathEscape(channelNo), ReturnURL: s.origin + "/payment/return/" + url.PathEscape(attemptNo), ClientIP: clientIP, OpenID: openID, ExpiresAt: expiresAt})
		return createErr
	})
	if err != nil || redirect == nil {
		_ = s.attempts.FailAndRelease(ctx, attemptNo)
		if err == nil {
			err = gerror.New("payment channel returned no redirect")
		}
		return nil, err
	}
	if err = validateRedirect(mode, *redirect); err != nil {
		_ = s.attempts.FailAndRelease(ctx, attemptNo)
		return nil, err
	}
	if err = s.attempts.MarkSubmitted(ctx, attemptNo); err != nil {
		_ = s.attempts.FailAndRelease(ctx, attemptNo)
		return nil, err
	}
	return &Created{AttemptNo: attemptNo, StatusCode: "confirming", Redirect: *redirect, ExpiresAt: expiresAt}, nil
}

func (s *Service) Status(ctx context.Context, attemptNo string) (*paymentattempt.Status, error) {
	actor, err := s.attempts.CurrentActor(ctx)
	if err != nil || actor == nil {
		return nil, gerror.New("payment attempt is not visible")
	}
	return s.attempts.ObserveReturn(ctx, attemptNo, actor.PrincipalRefDigest)
}

func (s *Service) AvailableChannels(ctx context.Context, currencyCode string) ([]channelconfig.AvailableChannel, error) {
	if _, err := s.attempts.CurrentActor(ctx); err != nil {
		return nil, gerror.New("payment channels are not visible")
	}
	return s.channels.ListAvailable(ctx, currencyCode)
}

func (s *Service) Callback(ctx context.Context, channelNo string, input paymentcap.CallbackRequest) (*paymentattempt.CallbackOutcome, paymentcap.ProviderCode, error) {
	var verified *paymentcap.VerifiedResult
	var provider paymentcap.ProviderCode
	err := s.channels.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		provider = adapter.Provider()
		var verifyErr error
		verified, verifyErr = adapter.VerifyCallback(ctx, credentials, input)
		return verifyErr
	})
	if err != nil || verified == nil {
		return nil, provider, err
	}
	payloadDigest, err := callbackDigest(input)
	if err != nil {
		return nil, provider, err
	}
	result, err := s.attempts.RecordVerifiedCallback(ctx, paymentattempt.CallbackInput{ChannelNo: channelNo, Result: *verified, PayloadDigest: payloadDigest})
	return result, provider, err
}

// RefundCallback verifies a channel callback and applies its monotonic result to the durable refund attempt.
func (s *Service) RefundCallback(ctx context.Context, channelNo string, input paymentcap.CallbackRequest) (*paymentattempt.RefundResultOutcome, paymentcap.ProviderCode, error) {
	var verified *paymentcap.VerifiedResult
	var provider paymentcap.ProviderCode
	err := s.channels.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		provider = adapter.Provider()
		var verifyErr error
		verified, verifyErr = adapter.VerifyRefundCallback(ctx, credentials, input)
		return verifyErr
	})
	if err != nil || verified == nil {
		return nil, provider, err
	}
	result, err := s.attempts.ApplyVerifiedRefundResult(ctx, channelNo, *verified, "refund_callback")
	return result, provider, err
}

func (s *Service) RunOneQuery(ctx context.Context) (bool, error) {
	work, err := s.attempts.ClaimQuery(ctx, time.Minute)
	if err != nil || work == nil {
		return false, err
	}
	var result *paymentcap.VerifiedResult
	err = s.channels.WithCredentials(ctx, work.ChannelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		var queryErr error
		result, queryErr = adapter.Query(ctx, credentials, paymentcap.QueryRequest{AttemptNo: work.AttemptNo, ExternalTradeReference: work.ExternalTradeReference})
		return queryErr
	})
	if err != nil {
		if retryErr := s.attempts.RetryQuery(ctx, *work); retryErr != nil {
			return true, retryErr
		}
		return true, err
	}
	if result == nil {
		if retryErr := s.attempts.RetryQuery(ctx, *work); retryErr != nil {
			return true, retryErr
		}
		return true, gerror.New("payment provider returned no query result")
	}
	return true, s.attempts.CompleteQuery(ctx, *work, *result)
}

func (s *Service) RunOneConfirmation(ctx context.Context) (bool, error) {
	work, err := s.attempts.ClaimConfirmation(ctx, time.Minute)
	if err != nil || work == nil {
		return false, err
	}
	return true, s.attempts.CompleteConfirmation(ctx, *work)
}

func (s *Service) RunOneRefund(ctx context.Context) (bool, error) {
	const workerNo = "payment-refund-worker"
	work, err := s.attempts.ClaimRefundExecution(ctx, workerNo, 60)
	if err != nil || work == nil || !work.HasWork {
		return false, err
	}
	payment, err := s.attempts.GetRefundPaymentContext(ctx, work.PaymentAttemptNo)
	if err != nil {
		return true, err
	}
	if payment.CurrencyCode != work.CurrencyCode || work.AmountMinor > payment.ChannelAmountMinor {
		evidenceDigest, digestErr := refundPaymentContextMismatchEvidence(*work, *payment)
		if digestErr != nil {
			return true, digestErr
		}
		if evidenceErr := s.attempts.RecordRefundEvidence(
			ctx, work.CommandNo, "refund_preflight", evidenceDigest, "payment_context_mismatch",
		); evidenceErr != nil {
			return true, evidenceErr
		}
		_, reportErr := s.attempts.ReportRefundExecution(ctx, billingcap.ReportRefundExecution{
			WorkerNo: workerNo, CommandNo: work.CommandNo, RequestDigest: work.RequestDigest,
			Phase: work.Phase, Status: billingcap.RefundExecutionResultManualConfirmation,
			ResultCode: "payment_context_mismatch", EvidenceDigest: evidenceDigest,
		})
		return true, reportErr
	}
	if work.Phase == billingcap.RefundExecutionPhasePreflight {
		capability, capabilityErr := s.channels.RefundCapability(
			ctx, payment.ChannelNo, paymentcap.Mode(payment.ModeCode), payment.CurrencyCode,
		)
		if capabilityErr != nil {
			return true, capabilityErr
		}
		status := billingcap.RefundExecutionResultReady
		resultCode := "preflight_ready"
		if !capability.Supported {
			status = billingcap.RefundExecutionResultFailed
			resultCode = "original_channel_unsupported"
		}
		if evidenceErr := s.attempts.RecordRefundEvidence(
			ctx, work.CommandNo, "refund_preflight", capability.EvidenceDigest, resultCode,
		); evidenceErr != nil {
			return true, evidenceErr
		}
		_, reportErr := s.attempts.ReportRefundExecution(ctx, billingcap.ReportRefundExecution{
			WorkerNo: workerNo, CommandNo: work.CommandNo, RequestDigest: work.RequestDigest,
			Phase: work.Phase, Status: status, ResultCode: resultCode,
			EvidenceDigest: capability.EvidenceDigest,
		})
		return true, reportErr
	}

	attempt, err := s.attempts.EnsureRefundAttempt(ctx, *work, *payment)
	if err != nil {
		return true, err
	}
	if isRefundTerminal(attempt.StatusCode) {
		return true, s.reportRefundTerminal(ctx, workerNo, *work, *attempt)
	}
	var result *paymentcap.VerifiedResult
	var providerErr error
	if attempt.StatusCode == "pending" {
		result, providerErr = s.submitRefund(ctx, *work, *payment, *attempt)
	} else {
		result, providerErr = s.queryRefund(ctx, *payment, *attempt)
	}
	return true, s.progressRefundResult(ctx, workerNo, *work, *attempt, result, providerErr)
}

func (s *Service) submitRefund(
	ctx context.Context,
	work billingcap.RefundExecutionWork,
	payment paymentattempt.RefundPaymentContext,
	attempt paymentattempt.RefundAttempt,
) (*paymentcap.VerifiedResult, error) {
	var result *paymentcap.VerifiedResult
	err := s.channels.WithCredentials(ctx, payment.ChannelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		var callErr error
		result, callErr = adapter.Refund(ctx, credentials, paymentcap.RefundRequest{
			RefundAttemptNo: attempt.RefundAttemptNo, ExternalTradeReference: payment.ExternalTradeReference,
			Currency: work.CurrencyCode, AmountMinor: work.AmountMinor, TotalAmountMinor: payment.TotalAmountMinor,
			NotifyURL: s.origin + "/api/v1/callbacks/refunds/" + url.PathEscape(payment.ChannelNo),
		})
		return callErr
	})
	return result, err
}

func (s *Service) queryRefund(
	ctx context.Context,
	payment paymentattempt.RefundPaymentContext,
	attempt paymentattempt.RefundAttempt,
) (*paymentcap.VerifiedResult, error) {
	var result *paymentcap.VerifiedResult
	err := s.channels.WithCredentials(ctx, payment.ChannelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		var queryErr error
		result, queryErr = adapter.QueryRefund(ctx, credentials, paymentcap.RefundQueryRequest{
			RefundAttemptNo: attempt.RefundAttemptNo, OriginalTradeReference: payment.ExternalTradeReference,
			ExternalRefundReference: attempt.ResultReference,
		})
		return queryErr
	})
	return result, err
}

func (s *Service) progressRefundResult(
	ctx context.Context,
	workerNo string,
	work billingcap.RefundExecutionWork,
	attempt paymentattempt.RefundAttempt,
	result *paymentcap.VerifiedResult,
	providerErr error,
) error {
	localStatus, resultCode := "submitted", "refund_processing"
	resultReference := ""
	if providerErr != nil || result == nil {
		resultCode = "provider_result_unknown"
	} else {
		resultReference = result.ExternalTradeReference
		switch {
		case result.AttemptNo != attempt.RefundAttemptNo || result.Currency != work.CurrencyCode || result.AmountMinor != work.AmountMinor:
			localStatus, resultCode = "manual_required", "provider_result_mismatch"
		case result.Status == "succeeded":
			localStatus, resultCode = "succeeded", "refund_succeeded"
		case result.Status == "failed" || result.Status == "closed":
			localStatus, resultCode = "failed", "provider_refund_failed"
		case result.Status == "manual_review":
			localStatus, resultCode = "manual_required", "provider_refund_manual_required"
		case result.Status != "pending":
			localStatus, resultCode = "manual_required", "provider_result_mismatch"
		}
	}
	if localStatus == "submitted" && work.Attempt >= 8 {
		localStatus, resultCode = "result_unknown", "provider_result_unknown"
	}
	evidenceDigest, err := refundExecutionEvidence(work, attempt.RefundAttemptNo, resultCode, result)
	if err != nil {
		return err
	}
	if err = s.attempts.CompleteRefundAttempt(
		ctx, attempt.RefundAttemptNo, localStatus, resultCode, resultReference, evidenceDigest,
	); err != nil {
		return err
	}
	if err = s.attempts.RecordRefundEvidence(
		ctx, attempt.RefundAttemptNo, "refund_result", evidenceDigest, resultCode,
	); err != nil {
		return err
	}
	if localStatus == "submitted" {
		return nil
	}
	attempt.StatusCode = localStatus
	attempt.ResultCode = resultCode
	attempt.ResultReference = resultReference
	attempt.EvidenceDigest = evidenceDigest
	return s.reportRefundTerminal(ctx, workerNo, work, attempt)
}

func (s *Service) reportRefundTerminal(
	ctx context.Context,
	workerNo string,
	work billingcap.RefundExecutionWork,
	attempt paymentattempt.RefundAttempt,
) error {
	status := billingcap.RefundExecutionResultStatus(attempt.StatusCode)
	if attempt.StatusCode == "manual_required" {
		status = billingcap.RefundExecutionResultManualConfirmation
	}
	_, err := s.attempts.ReportRefundExecution(ctx, billingcap.ReportRefundExecution{
		WorkerNo: workerNo, CommandNo: work.CommandNo, RequestDigest: work.RequestDigest,
		Phase: work.Phase, Status: status, ResultCode: attempt.ResultCode,
		ResultReference: attempt.ResultReference, EvidenceDigest: attempt.EvidenceDigest,
	})
	return err
}

func isRefundTerminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "result_unknown" || status == "manual_required"
}

func refundPaymentContextMismatchEvidence(
	work billingcap.RefundExecutionWork,
	payment paymentattempt.RefundPaymentContext,
) (string, error) {
	value := struct {
		CommandNo, PaymentAttemptNo, WorkCurrency, PaymentCurrency string
		WorkAmountMinor, ChannelAmountMinor                        int64
	}{
		work.CommandNo, work.PaymentAttemptNo, work.CurrencyCode, payment.CurrencyCode,
		work.AmountMinor, payment.ChannelAmountMinor,
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

// RunOnePaymentClosure converges one pending core cancellation. The payment
// plugin closes only its own attempts and reports evidence to the core owner.
func (s *Service) RunOnePaymentClosure(ctx context.Context) (bool, error) {
	work, err := s.attempts.ClaimPaymentClosure(ctx)
	if err != nil {
		return false, err
	}
	if work == nil || !work.Found {
		return false, nil
	}
	outcome, reference, digest, err := s.attempts.CloseOrderAttempts(ctx, work.OrderNo, func(callCtx context.Context, attempt paymentattempt.ClosureAttempt) error {
		return s.channels.WithCredentials(callCtx, attempt.ChannelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
			if !adapter.Capabilities().Operations[paymentcap.OperationClose] {
				return gerror.New("payment close capability is unavailable")
			}
			return adapter.Close(callCtx, credentials, paymentattempt.ProviderCloseRequest(attempt))
		})
	})
	if err != nil {
		return true, err
	}
	_, err = s.attempts.ReportPaymentClosure(ctx, billingcap.ReportPaymentClosure{CancellationNo: work.CancellationNo, OutcomeCode: outcome, Reference: reference, EvidenceDigest: digest})
	return true, err
}

func refundExecutionEvidence(
	work billingcap.RefundExecutionWork,
	refundAttemptNo string,
	resultCode string,
	result *paymentcap.VerifiedResult,
) (string, error) {
	value := struct {
		CommandNo, RefundNo, RefundSourceNo, PaymentAttemptNo string
		RefundAttemptNo, CurrencyCode, ResultCode             string
		AmountMinor                                           int64
		ProviderResult                                        *paymentcap.VerifiedResult
	}{
		work.CommandNo, work.RefundNo, work.RefundSourceNo, work.PaymentAttemptNo,
		refundAttemptNo, work.CurrencyCode, resultCode, work.AmountMinor, result,
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func validateRedirect(mode paymentcap.Mode, redirect paymentcap.Redirect) error {
	switch mode {
	case paymentcap.ModeWechatJSAPI:
		if redirect.Method != "JSAPI" || redirect.URL != "" || redirect.Fields["paySign"] == "" {
			return gerror.New("unsafe payment redirect")
		}
	case paymentcap.ModeWechatNative:
		parsed, err := url.Parse(redirect.URL)
		if err != nil || parsed.Scheme != "weixin" || redirect.Method != "QR" {
			return gerror.New("unsafe payment redirect")
		}
	case paymentcap.ModeWechatH5:
		parsed, err := url.Parse(redirect.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host != "wx.tenpay.com" || redirect.Method != http.MethodGet {
			return gerror.New("unsafe payment redirect")
		}
	default:
		parsed, err := url.Parse(redirect.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || redirect.Method != http.MethodPost {
			return gerror.New("unsafe payment redirect")
		}
	}
	return nil
}

func requestDigest(input any) (string, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func callbackDigest(input paymentcap.CallbackRequest) (string, error) {
	if len(input.Body) > 0 {
		sum := sha256.Sum256(input.Body)
		return hex.EncodeToString(sum[:]), nil
	}
	raw, err := json.Marshal(input.Values)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func randomNo(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}
