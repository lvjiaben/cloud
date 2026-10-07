package paymentattempt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type ClosureAttempt struct{ AttemptNo, ChannelNo, ExternalTradeReference, StatusCode string }
type CloseAttempt func(context.Context, ClosureAttempt) error

var errClosureInProgress = gerror.New("payment closure is already in progress")

func (s *Store) ClaimPaymentClosure(ctx context.Context) (*billingcap.PaymentClosureWork, error) {
	return s.billing.ClaimPaymentClosure(ctx)
}
func (s *Store) ReportPaymentClosure(ctx context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	return s.billing.ReportPaymentClosure(ctx, input)
}

// CloseOrderAttempts closes every unfinished attempt and returns one bounded
// outcome for the core cancellation handshake.
func (s *Store) CloseOrderAttempts(ctx context.Context, orderNo string, closeProvider CloseAttempt) (outcome, reference, evidenceDigest string, err error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || closeProvider == nil {
		return "", "", "", gerror.New("payment closure request is invalid")
	}
	rows, err := s.database.GetAll(ctx, `SELECT attempt_no,channel_no,external_trade_reference,status_code FROM plugin_idc_payment_attempt WHERE tenant_id=0 AND order_no=? ORDER BY id`, orderNo)
	if err != nil {
		return "", "", "", err
	}
	if len(rows) == 0 {
		digest := closureDigest(orderNo, "no_attempts")
		return "no_attempts", "pcl_" + digest[:32], digest, nil
	}
	states := make([]string, 0, len(rows))
	outcome = "closed"
	for _, row := range rows {
		attempt := ClosureAttempt{AttemptNo: row["attempt_no"].String(), ChannelNo: row["channel_no"].String(), ExternalTradeReference: row["external_trade_reference"].String(), StatusCode: row["status_code"].String()}
		switch attempt.StatusCode {
		case "succeeded", "late_success":
			outcome = "confirmed_success"
		case "manual_review", "disputed":
			if outcome != "confirmed_success" {
				outcome = "unknown"
			}
		case "created":
			if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='closed',updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code='created'`, attempt.AttemptNo); err != nil {
				return "", "", "", err
			}
			attempt.StatusCode = "closed"
		case "submitted", "confirming":
			claim, claimErr := s.claimClosureCall(ctx, orderNo, attempt.AttemptNo)
			if claimErr != nil {
				return "", "", "", claimErr
			}
			switch claim {
			case "closed":
				attempt.StatusCode = "closed"
				states = append(states, attempt.AttemptNo+":"+attempt.StatusCode)
				continue
			case "result_unknown":
				if outcome != "confirmed_success" {
					outcome = "unknown"
				}
				states = append(states, attempt.AttemptNo+":result_unknown")
				continue
			}
			if closeErr := closeProvider(ctx, attempt); closeErr != nil {
				if err = s.finishClosureCall(ctx, attempt.AttemptNo, false); err != nil {
					return "", "", "", err
				}
				if outcome != "confirmed_success" {
					outcome = "unknown"
				}
				attempt.StatusCode = "result_unknown"
			} else if err = s.finishClosureCall(ctx, attempt.AttemptNo, true); err != nil {
				return "", "", "", err
			} else {
				attempt.StatusCode = "closed"
			}
		}
		states = append(states, attempt.AttemptNo+":"+attempt.StatusCode)
	}
	sort.Strings(states)
	digest := closureDigest(orderNo, outcome, strings.Join(states, ","))
	return outcome, "pcl_" + digest[:32], digest, nil
}

// claimClosureCall creates a durable fence before crossing the provider
// boundary. A recent caller is never replayed. A stale caller is converted to
// result_unknown so an operator can reconcile it safely.
func (s *Store) claimClosureCall(ctx context.Context, orderNo, attemptNo string) (string, error) {
	digest := closureDigest(orderNo, attemptNo)
	operationNo := "pco_" + digest[:32]
	if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_closure_operation(operation_no,order_no,attempt_no,result_evidence_digest) VALUES(?,?,?,?) ON CONFLICT(attempt_no) DO NOTHING`, operationNo, orderNo, attemptNo, digest); err != nil {
		return "", err
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_closure_operation SET status_code='calling',started_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code='prepared'`, attemptNo)
	if err != nil {
		return "", err
	}
	if affected, _ := result.RowsAffected(); affected == 1 {
		return "calling", nil
	}
	row, err := s.database.GetOne(ctx, `SELECT status_code,updated_at FROM plugin_idc_payment_closure_operation WHERE attempt_no=?`, attemptNo)
	if err != nil || row.IsEmpty() {
		return "", err
	}
	status := row["status_code"].String()
	if status == "calling" && row["updated_at"].Time().After(time.Now().Add(-2*time.Minute)) {
		return "", errClosureInProgress
	}
	if status == "calling" {
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_payment_closure_operation SET status_code='result_unknown',finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code='calling'`, attemptNo); err != nil {
			return "", err
		}
		return "result_unknown", nil
	}
	return status, nil
}

func (s *Store) finishClosureCall(ctx context.Context, attemptNo string, closed bool) error {
	status := "result_unknown"
	if closed {
		status = "closed"
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		result, err := tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_closure_operation SET status_code=?,finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code='calling'`, status, attemptNo)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return gerror.New("payment closure fencing state changed")
		}
		if !closed {
			return nil
		}
		_, err = tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_attempt SET status_code='closed',result_unknown=FALSE,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code IN ('submitted','confirming')`, attemptNo)
		return err
	})
}

func ProviderCloseRequest(attempt ClosureAttempt) paymentcap.CloseRequest {
	return paymentcap.CloseRequest{AttemptNo: attempt.AttemptNo, ExternalTradeReference: attempt.ExternalTradeReference}
}
func closureDigest(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}
