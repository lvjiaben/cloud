package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

var privateCreditEvidencePattern = regexp.MustCompile(`^private:[A-Za-z0-9][A-Za-z0-9._/-]{0,119}$`)

type creditPolicyCommand struct {
	CreditPolicyInput
	PolicyNo      string
	ContentDigest string
}

type creditApplicationCommand struct {
	CreditApplicationInput
	ApplicationNo         string
	RequiredApprovalCount int
	RequestDigest         string
}

func normalizeCreditPolicy(input CreditPolicyInput) (creditPolicyCommand, error) {
	if !exactlyTrimmed(input.CurrencyCode, input.IdempotencyKey) {
		return creditPolicyCommand{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.MaximumLimitMinor) ||
		input.DualReviewThresholdMinor <= 0 ||
		input.DualReviewThresholdMinor > input.MaximumLimitMinor ||
		input.BillingDay < 1 || input.BillingDay > 28 ||
		input.RepaymentPeriodDays < 1 || input.RepaymentPeriodDays > 90 ||
		input.GracePeriodDays < 0 || input.GracePeriodDays > 90 ||
		input.DailyLateFeeBasisPoints < 0 || input.DailyLateFeeBasisPoints > 10000 ||
		input.MaximumLateFeeMinor < 0 || input.MaximumLateFeeMinor > maximumAmountMinor ||
		(input.DailyLateFeeBasisPoints == 0) != (input.MaximumLateFeeMinor == 0) ||
		input.ConfiguredBy <= 0 || !validCreditIdempotencyKey(input.IdempotencyKey) {
		return creditPolicyCommand{}, bizerr.NewCode(CodeCreditInvalid)
	}
	command := creditPolicyCommand{
		CreditPolicyInput: input,
		PolicyNo:          deterministicCreditNo("crp_", "policy", input.CurrencyCode, input.IdempotencyKey),
	}
	digest, err := contentDigest(struct {
		PolicyNo                 string
		CurrencyCode             string
		MaximumLimitMinor        int64
		DualReviewThresholdMinor int64
		BillingDay               int
		RepaymentPeriodDays      int
		GracePeriodDays          int
		DailyLateFeeBasisPoints  int
		MaximumLateFeeMinor      int64
		ConfiguredBy             int
	}{
		command.PolicyNo, input.CurrencyCode, input.MaximumLimitMinor,
		input.DualReviewThresholdMinor, input.BillingDay, input.RepaymentPeriodDays,
		input.GracePeriodDays, input.DailyLateFeeBasisPoints,
		input.MaximumLateFeeMinor, input.ConfiguredBy,
	})
	if err != nil {
		return creditPolicyCommand{}, err
	}
	command.ContentDigest = digest
	return command, nil
}

func normalizeCreditApplication(input CreditApplicationInput) (creditApplicationCommand, error) {
	if !exactlyTrimmed(
		input.CustomerNo, input.CurrencyCode, input.RiskEvidenceReference,
		input.RiskEvidenceDigest, input.RequestActorType, input.IdempotencyKey,
	) {
		return creditApplicationCommand{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.RiskEvidenceReference = strings.TrimSpace(input.RiskEvidenceReference)
	input.RiskEvidenceDigest = strings.TrimSpace(input.RiskEvidenceDigest)
	input.RequestActorType = strings.TrimSpace(input.RequestActorType)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.RequestedValidFrom = input.RequestedValidFrom.UTC().Truncate(time.Microsecond)
	input.RequestedValidUntil = input.RequestedValidUntil.UTC().Truncate(time.Microsecond)
	if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.RequestedLimitMinor) ||
		input.RequestedValidFrom.IsZero() || input.RequestedValidUntil.IsZero() ||
		!input.RequestedValidUntil.After(input.RequestedValidFrom) ||
		!privateCreditEvidencePattern.MatchString(input.RiskEvidenceReference) ||
		!canonicalDigestPattern.MatchString(input.RiskEvidenceDigest) ||
		(input.RequestActorType != "customer" && input.RequestActorType != "staff") ||
		input.RequestedBy <= 0 || !validCreditIdempotencyKey(input.IdempotencyKey) {
		return creditApplicationCommand{}, bizerr.NewCode(CodeCreditInvalid)
	}
	command := creditApplicationCommand{
		CreditApplicationInput: input,
		ApplicationNo: deterministicCreditNo(
			"cra_", "application", input.CustomerNo, input.CurrencyCode, input.IdempotencyKey,
		),
	}
	digest, err := contentDigest(struct {
		ApplicationNo         string
		CustomerNo            string
		CurrencyCode          string
		RequestedLimitMinor   int64
		RequestedValidFrom    time.Time
		RequestedValidUntil   time.Time
		RiskEvidenceReference string
		RiskEvidenceDigest    string
		RequestActorType      string
		RequestedBy           int
	}{
		command.ApplicationNo, input.CustomerNo, input.CurrencyCode,
		input.RequestedLimitMinor, input.RequestedValidFrom,
		input.RequestedValidUntil, input.RiskEvidenceReference,
		input.RiskEvidenceDigest, input.RequestActorType, input.RequestedBy,
	})
	if err != nil {
		return creditApplicationCommand{}, err
	}
	command.RequestDigest = digest
	return command, nil
}

func normalizeCreditReview(input CreditReviewInput) (CreditReviewInput, error) {
	if !exactlyTrimmed(input.ApplicationNo, input.CommentDigest, input.EvidenceDigest) {
		return CreditReviewInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.ApplicationNo = strings.TrimSpace(input.ApplicationNo)
	input.CommentDigest = strings.TrimSpace(input.CommentDigest)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	if !validBusinessReference(input.ApplicationNo, 40) ||
		(input.Decision != CreditDecisionApprove && input.Decision != CreditDecisionReject) ||
		input.ReviewerID <= 0 || !canonicalDigestPattern.MatchString(input.CommentDigest) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) {
		return CreditReviewInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditReserve(input CreditReserveInput) (CreditReserveInput, error) {
	if !exactlyTrimmed(
		input.CustomerNo, input.CurrencyCode, input.SourceType,
		input.SourceReference, input.OrderNo, input.InvoiceNo,
		input.SourceSnapshotDigest, input.ActorType,
	) {
		return CreditReserveInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.InvoiceNo = strings.TrimSpace(input.InvoiceNo)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	input.ActorType = strings.TrimSpace(input.ActorType)
	allowedSource := input.SourceType == "order" || input.SourceType == "renewal" ||
		input.SourceType == "change" || input.SourceType == "manual" ||
		input.SourceType == "migration"
	if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) || !validAmount(input.AmountMinor) ||
		!allowedSource || !validBusinessReference(input.SourceReference, 128) ||
		(input.SourceType == "order" && !validBusinessReference(input.OrderNo, 40)) ||
		(input.SourceType != "order" && input.OrderNo != "") ||
		(input.InvoiceNo != "" && !validBusinessReference(input.InvoiceNo, 40)) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) ||
		!validCreditActor(input.ActorType, input.ActorID) {
		return CreditReserveInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditRelease(input CreditReleaseInput) (CreditReleaseInput, error) {
	if !exactlyTrimmed(
		input.ExposureNo, input.SourceReference, input.EvidenceDigest, input.ActorType,
	) {
		return CreditReleaseInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.ExposureNo = strings.TrimSpace(input.ExposureNo)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	input.ActorType = strings.TrimSpace(input.ActorType)
	if !validBusinessReference(input.ExposureNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) ||
		!validCreditActor(input.ActorType, input.ActorID) {
		return CreditReleaseInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditStatementIssue(
	input CreditStatementIssueInput,
) (CreditStatementIssueInput, error) {
	if !exactlyTrimmed(input.CreditAccountNo, input.IdempotencyKey) {
		return CreditStatementIssueInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.CreditAccountNo = strings.TrimSpace(input.CreditAccountNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.PeriodStartedAt = input.PeriodStartedAt.UTC().Truncate(time.Microsecond)
	input.PeriodEndedAt = input.PeriodEndedAt.UTC().Truncate(time.Microsecond)
	if !validBusinessReference(input.CreditAccountNo, 40) ||
		!validCreditIdempotencyKey(input.IdempotencyKey) ||
		input.PeriodStartedAt.IsZero() || input.PeriodEndedAt.IsZero() ||
		!input.PeriodEndedAt.After(input.PeriodStartedAt) ||
		input.PeriodEndedAt.Sub(input.PeriodStartedAt) > 62*24*time.Hour {
		return CreditStatementIssueInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditPrepayment(input CreditPrepaymentInput) (CreditPrepaymentInput, error) {
	if !exactlyTrimmed(
		input.ExposureNo, input.FundingSourceType, input.FundingSourceReference,
		input.LedgerTransactionNo, input.SourceSnapshotDigest,
		input.ConfirmationEvidenceDigest,
	) {
		return CreditPrepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.ExposureNo = strings.TrimSpace(input.ExposureNo)
	input.FundingSourceType = strings.TrimSpace(input.FundingSourceType)
	input.FundingSourceReference = strings.TrimSpace(input.FundingSourceReference)
	input.LedgerTransactionNo = strings.TrimSpace(input.LedgerTransactionNo)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	input.ConfirmationEvidenceDigest = strings.TrimSpace(input.ConfirmationEvidenceDigest)
	allowedFunding := input.FundingSourceType == "online_payment" ||
		input.FundingSourceType == "customer_balance" ||
		input.FundingSourceType == "manual_receipt" ||
		input.FundingSourceType == "migration"
	if !validBusinessReference(input.ExposureNo, 40) || !allowedFunding ||
		!validBusinessReference(input.FundingSourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) ||
		!validAmount(input.AmountMinor) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(input.ConfirmationEvidenceDigest) {
		return CreditPrepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditStatementRepayment(
	input CreditStatementRepaymentInput,
) (CreditStatementRepaymentInput, error) {
	if !exactlyTrimmed(
		input.StatementNo, input.FundingSourceType, input.FundingSourceReference,
		input.LedgerTransactionNo, input.SourceSnapshotDigest,
		input.ConfirmationEvidenceDigest,
	) {
		return CreditStatementRepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.StatementNo = strings.TrimSpace(input.StatementNo)
	input.FundingSourceType = strings.TrimSpace(input.FundingSourceType)
	input.FundingSourceReference = strings.TrimSpace(input.FundingSourceReference)
	input.LedgerTransactionNo = strings.TrimSpace(input.LedgerTransactionNo)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	input.ConfirmationEvidenceDigest = strings.TrimSpace(input.ConfirmationEvidenceDigest)
	allowedFunding := input.FundingSourceType == "online_payment" ||
		input.FundingSourceType == "customer_balance" ||
		input.FundingSourceType == "manual_receipt" ||
		input.FundingSourceType == "migration"
	if !validBusinessReference(input.StatementNo, 40) || !allowedFunding ||
		!validBusinessReference(input.FundingSourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) ||
		!validAmount(input.AmountMinor) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(input.ConfirmationEvidenceDigest) {
		return CreditStatementRepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func normalizeCreditBalanceRepayment(
	input CreditBalanceRepaymentInput,
) (CreditBalanceRepaymentInput, error) {
	if !exactlyTrimmed(
		input.StatementNo, input.SourceReference, input.SourceSnapshotDigest,
		input.ConfirmationEvidenceDigest,
	) {
		return CreditBalanceRepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.StatementNo = strings.TrimSpace(input.StatementNo)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	input.ConfirmationEvidenceDigest = strings.TrimSpace(input.ConfirmationEvidenceDigest)
	if !validBusinessReference(input.StatementNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!validAmount(input.AmountMinor) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(input.ConfirmationEvidenceDigest) {
		return CreditBalanceRepaymentInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func validCreditActor(actorType string, actorID int) bool {
	if actorType == "system" {
		return actorID == 0
	}
	return (actorType == "customer" || actorType == "staff") && actorID > 0
}

func validCreditIdempotencyKey(value string) bool {
	return len(value) >= 8 && validBusinessReference(value, 128)
}

func deterministicCreditNo(prefix string, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + hex.EncodeToString(digest[:16])
}
