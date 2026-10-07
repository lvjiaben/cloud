// This file creates and advances durable refund execution commands.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

// maximumRefundBatchRows bounds one SQL statement well below PostgreSQL's
// parameter limit while keeping persistence independent of item count.
const maximumRefundBatchRows = 250

// refundExecutionCommandInsert holds one fully prepared immutable command row.
type refundExecutionCommandInsert struct {
	commandNo       string
	refundNo        string
	sourceNo        string
	phase           string
	destination     string
	currency        string
	amountMinor     int64
	sourceReference string
	digest          string
	now             time.Time
}

type refundReplayRow struct {
	RefundNo                       string
	OrderNo                        string
	InvoiceNo                      string
	CurrencyCode                   string
	Status                         string
	Reason                         RefundReason
	ReasonDetail                   string
	AmountMinor                    int64
	ApprovalMode                   string
	Destination                    string
	RequiresEntitlementTermination bool
	HasResourceImpact              bool
	HasDangerousOperation          bool
	HasFundAnomaly                 bool
	HighAmount                     bool
	Version                        int64
	ImpactDigest                   string
	SubmittedAt                    time.Time
	ApprovedAt                     *time.Time
	CompletedAt                    *time.Time
}

func (s *onlineRefundService) loadRefundReplay(
	ctx context.Context, customerNo, idempotencyDigest string,
) (*OnlineRefund, error) {
	var row *refundReplayRow
	err := s.database.GetScan(ctx, &row, `
SELECT refund_no,order_no,invoice_no,currency_code,status_code AS status,
       reason_code AS reason,reason_detail,requested_amount_minor AS amount_minor,
       approval_mode_code AS approval_mode,refund_destination_code AS destination,
       requires_entitlement_termination,has_resource_impact,has_dangerous_operation,
       has_fund_anomaly,high_amount,version,impact_digest,
       submitted_at,approved_at,completed_at
FROM plugin_idc_core_refund_request
WHERE tenant_id=0 AND customer_no=? AND idempotency_digest=?`,
		customerNo, idempotencyDigest)
	if err != nil || row == nil {
		return nil, err
	}
	return &OnlineRefund{
		RefundNo: row.RefundNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo,
		CurrencyCode: row.CurrencyCode, Status: row.Status, Reason: row.Reason,
		ReasonDetail: row.ReasonDetail, AmountMinor: row.AmountMinor,
		ApprovalMode: row.ApprovalMode, Destination: row.Destination,
		RequiresEntitlementTermination: row.RequiresEntitlementTermination,
		HasResourceImpact:              row.HasResourceImpact,
		HasDangerousOperation:          row.HasDangerousOperation,
		HasFundAnomaly:                 row.HasFundAnomaly, HighAmount: row.HighAmount,
		Version: row.Version, ImpactDigest: row.ImpactDigest,
		SubmittedAt: row.SubmittedAt.UTC(), ApprovedAt: refundUTCPointer(row.ApprovedAt),
		CompletedAt: refundUTCPointer(row.CompletedAt),
	}, nil
}

func (s *onlineRefundService) insertRefundRequest(
	ctx context.Context,
	customerUserID int,
	preview *RefundPreview,
	reason RefundReason,
	reasonDetail string,
	idempotencyDigest string,
	requestDigest string,
) (*OnlineRefund, error) {
	if preview == nil || customerUserID <= 0 {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	now, err := refundDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	refundNo, err := newBillingNumber(s.random, "rfd_")
	if err != nil {
		return nil, err
	}
	status := "pending_review"
	var approvedAt *time.Time
	if preview.ApprovalMode == "automatic" {
		status = "approved"
		approvedAt = &now
	}
	var policyNo any
	var policyVersion any
	if preview.PolicyNo != "" {
		policyNo = preview.PolicyNo
		policyVersion = preview.PolicyVersion
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_refund_request
    (tenant_id,refund_no,customer_no,order_no,invoice_no,currency_code,
     status_code,reason_code,reason_detail,requested_amount_minor,
     approval_mode_code,refund_destination_code,requires_entitlement_termination,
     has_resource_impact,has_dangerous_operation,has_fund_anomaly,high_amount,
     policy_no,policy_version,impact_digest,request_digest,idempotency_digest,
     version,submitted_at,approved_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?,?)`,
		refundNo, preview.CustomerNo, preview.OrderNo, preview.InvoiceNo,
		preview.CurrencyCode, status, reason, reasonDetail, preview.AmountMinor,
		preview.ApprovalMode, "original_channel", preview.RequiresEntitlementTermination,
		preview.HasResourceImpact, preview.HasDangerousOperation, preview.HasFundAnomaly,
		preview.HighAmount, policyNo, policyVersion, preview.ImpactDigest,
		requestDigest, idempotencyDigest, now, approvedAt, now); err != nil {
		return nil, err
	}

	itemRows := make([][]any, 0, len(preview.Items))
	impactRows := make([][]any, 0)
	lockRows := make([][]any, 0)
	sourceRows := make([][]any, 0)
	commandRows := make([]refundExecutionCommandInsert, 0)
	for index, item := range preview.Items {
		refundItemNo, numberErr := newBillingNumber(s.random, "rfi_")
		if numberErr != nil {
			return nil, numberErr
		}
		itemRows = append(itemRows, []any{0, refundItemNo, refundNo, index + 1,
			preview.InvoiceNo, item.InvoiceItemNo, item.OrderItemNo, preview.CustomerNo,
			preview.CurrencyCode, item.Quantity, item.AmountMinor,
			item.RefundableAmountMinor, item.ImpactDigest, now})
		for _, impact := range item.Impacts {
			impactNo, numberErr := newBillingNumber(s.random, "rim_")
			if numberErr != nil {
				return nil, numberErr
			}
			impactRows = append(impactRows, []any{0, impactNo, refundNo, refundItemNo,
				impact.OrderItemNo, preview.CustomerNo, preview.CurrencyCode,
				impact.EntitlementNo, impact.EntitlementVersion, impact.StatusCode,
				impact.RequiresTermination, impact.ImpactDigest, now})
			lockNo, numberErr := newBillingNumber(s.random, "rfl_")
			if numberErr != nil {
				return nil, numberErr
			}
			lockRows = append(lockRows, []any{0, lockNo, refundNo, refundItemNo,
				impactNo, impact.EntitlementNo, impact.EntitlementVersion, now})
		}
		for _, source := range item.Sources {
			sourceNo, numberErr := newBillingNumber(s.random, "rfs_")
			if numberErr != nil {
				return nil, numberErr
			}
			sourceRows = append(sourceRows, []any{0, sourceNo, refundNo, refundItemNo,
				refundNullableString(source.PaymentAllocationNo), source.PaymentSourceType,
				source.PaymentSourceReference, source.AmountMinor, source.ExecutionOwner,
				refundNullableString(source.CreditExposureNo),
				refundNullableString(string(source.CreditComponent)),
				refundNullableString(source.CreditRepaymentAllocationNo),
				refundNullableString(source.CreditPlanDigest), source.ContentDigest, now})
			if status == "approved" {
				command, commandErr := s.newRefundExecutionCommand(
					refundNo, sourceNo, preview.CurrencyCode, source, "original_channel", now,
				)
				if commandErr != nil {
					return nil, commandErr
				}
				commandRows = append(commandRows, command)
			}
		}
	}
	for _, batch := range []struct {
		name   string
		prefix string
		rows   [][]any
	}{
		{"refund request items", `INSERT INTO plugin_idc_core_refund_request_item
    (tenant_id,refund_item_no,refund_no,line_number,invoice_no,invoice_item_no,
     order_item_no,customer_no,currency_code,quantity,requested_amount_minor,
     refundable_amount_minor_snapshot,impact_digest,created_at) VALUES `, itemRows},
		{"refund entitlement impacts", `INSERT INTO plugin_idc_core_refund_entitlement_impact
    (tenant_id,impact_no,refund_no,refund_item_no,order_item_no,customer_no,
     currency_code,entitlement_no,entitlement_version_snapshot,status_snapshot,
     requires_termination,impact_digest,created_at) VALUES `, impactRows},
		{"refund entitlement locks", `INSERT INTO plugin_idc_core_refund_entitlement_lock
    (tenant_id,lock_no,refund_no,refund_item_no,impact_no,entitlement_no,
     expected_entitlement_version,created_at) VALUES `, lockRows},
		{"refund funding sources", `INSERT INTO plugin_idc_core_refund_source_allocation
    (tenant_id,refund_source_no,refund_no,refund_item_no,payment_allocation_no,
     payment_source_type_code,payment_source_reference,amount_minor,
     execution_owner_code,credit_exposure_no,credit_component_code,
     credit_repayment_allocation_no,credit_plan_digest,content_digest,created_at) VALUES `, sourceRows},
	} {
		if err = insertRefundRows(ctx, s.database, batch.prefix, batch.rows); err != nil {
			return nil, gerror.Wrap(err, "insert "+batch.name)
		}
	}
	if status == "approved" {
		if err = s.insertRefundExecutionCommands(ctx, commandRows); err != nil {
			return nil, err
		}
	}
	if status == "approved" {
		if err = s.insertRefundDecision(
			ctx, refundNo, 1, "automatic_approve", "", "approved",
			"system", 0, "rules_clear_unprovisioned", "", requestDigest, now,
		); err != nil {
			return nil, err
		}
	}
	return &OnlineRefund{
		RefundNo: refundNo, OrderNo: preview.OrderNo, InvoiceNo: preview.InvoiceNo,
		CurrencyCode: preview.CurrencyCode, Status: status, Reason: reason,
		ReasonDetail: reasonDetail, AmountMinor: preview.AmountMinor,
		ApprovalMode: preview.ApprovalMode, Destination: "original_channel",
		RequiresEntitlementTermination: preview.RequiresEntitlementTermination,
		HasResourceImpact:              preview.HasResourceImpact,
		HasDangerousOperation:          preview.HasDangerousOperation,
		HasFundAnomaly:                 preview.HasFundAnomaly, HighAmount: preview.HighAmount,
		Version: 1, ImpactDigest: preview.ImpactDigest, SubmittedAt: now,
		ApprovedAt: approvedAt,
	}, nil
}

// newRefundExecutionCommand prepares identifiers and immutable content evidence
// without performing a database round trip.
func (s *onlineRefundService) newRefundExecutionCommand(
	refundNo string,
	sourceNo string,
	currency string,
	source RefundSourceAllocation,
	destination string,
	now time.Time,
) (refundExecutionCommandInsert, error) {
	commandNo, err := newBillingNumber(s.random, "rfc_")
	if err != nil {
		return refundExecutionCommandInsert{}, err
	}
	digest, err := contentDigest(struct {
		RefundNo        string `json:"refundNo"`
		SourceNo        string `json:"sourceNo"`
		Destination     string `json:"destination"`
		CurrencyCode    string `json:"currencyCode"`
		AmountMinor     int64  `json:"amountMinor"`
		SourceReference string `json:"sourceReference"`
	}{
		refundNo, sourceNo, destination, currency,
		source.AmountMinor, source.PaymentSourceReference,
	})
	if err != nil {
		return refundExecutionCommandInsert{}, err
	}
	phase := "execution"
	if source.ExecutionOwner == "payment" && destination == "original_channel" {
		phase = "preflight"
	}
	return refundExecutionCommandInsert{
		commandNo: commandNo, refundNo: refundNo, sourceNo: sourceNo, phase: phase,
		destination: destination, currency: currency, amountMinor: source.AmountMinor,
		sourceReference: source.PaymentSourceReference, digest: digest, now: now,
	}, nil
}

// insertRefundExecutionCommands persists a bounded set of prepared commands in
// fixed-size batches instead of issuing one statement per funding source.
func (s *onlineRefundService) insertRefundExecutionCommands(
	ctx context.Context,
	commands []refundExecutionCommandInsert,
) error {
	rows := make([][]any, 0, len(commands))
	for _, command := range commands {
		rows = append(rows, []any{0, command.commandNo, command.refundNo, command.sourceNo,
			"pending", command.phase, command.destination, command.currency,
			command.amountMinor, command.sourceReference, command.digest, 0,
			command.now, command.now})
	}
	return insertRefundRows(ctx, s.database, `INSERT INTO plugin_idc_core_refund_execution_command
    (tenant_id,command_no,refund_no,refund_source_no,status_code,phase_code,
     destination_code,currency_code,amount_minor,source_reference,request_digest,
     attempt_count,created_at,updated_at) VALUES `, rows)
}

// insertRefundRows executes a bounded number of collection inserts. An empty
// collection is a no-op so optional entitlement impacts remain valid.
func insertRefundRows(
	ctx context.Context,
	database gdb.DB,
	prefix string,
	rows [][]any,
) error {
	for start := 0; start < len(rows); start += maximumRefundBatchRows {
		end := start + maximumRefundBatchRows
		if end > len(rows) {
			end = len(rows)
		}
		statement, args, err := buildRefundBatchStatement(prefix, rows[start:end])
		if err != nil {
			return err
		}
		if _, err = database.Exec(ctx, statement, args...); err != nil {
			return err
		}
	}
	return nil
}

// buildRefundBatchStatement creates one parameterized VALUES statement for a
// homogeneous row set and rejects malformed row widths before database access.
func buildRefundBatchStatement(prefix string, rows [][]any) (string, []any, error) {
	if strings.TrimSpace(prefix) == "" || len(rows) == 0 || len(rows[0]) == 0 {
		return "", nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	columnCount := len(rows[0])
	args := make([]any, 0, len(rows)*columnCount)
	var statement strings.Builder
	statement.Grow(len(prefix) + len(rows)*(columnCount*2+3))
	statement.WriteString(prefix)
	for index, row := range rows {
		if len(row) != columnCount {
			return "", nil, bizerr.NewCode(CodeRefundIntegrity)
		}
		if index > 0 {
			statement.WriteByte(',')
		}
		statement.WriteByte('(')
		for column := range row {
			if column > 0 {
				statement.WriteByte(',')
			}
			statement.WriteByte('?')
		}
		statement.WriteByte(')')
		args = append(args, row...)
	}
	return statement.String(), args, nil
}

func (s *onlineRefundService) insertRefundDecision(
	ctx context.Context,
	refundNo string,
	sequence int,
	decisionType string,
	fromStatus string,
	toStatus string,
	actorType string,
	actorID int,
	reasonCode string,
	commentDigest string,
	evidenceDigest string,
	now time.Time,
) error {
	decisionNo, err := newBillingNumber(s.random, "rdd_")
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_refund_decision
    (tenant_id,decision_no,refund_no,sequence_no,decision_type_code,
     from_status_code,to_status_code,actor_type_code,actor_id,reason_code,
     comment_digest,evidence_digest,created_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?)`, decisionNo, refundNo, sequence,
		decisionType, refundNullableString(fromStatus), toStatus, actorType, actorID,
		reasonCode, refundNullableString(commentDigest), evidenceDigest, now)
	return err
}

func refundDatabaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, err
	}
	parsed := value.Time()
	if parsed.IsZero() {
		return time.Time{}, bizerr.NewCode(CodeRefundIntegrity)
	}
	return parsed.UTC(), nil
}

func refundNullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func refundUTCPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
