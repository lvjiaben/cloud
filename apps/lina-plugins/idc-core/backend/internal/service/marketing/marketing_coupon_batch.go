// This file generates one-time coupon codes without retaining their plaintext.

package marketing

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/xuri/excelize/v2"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	maximumCouponBatchSize = 1000
	couponAlphabet         = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type CouponBatchGenerateInput struct {
	RequestNo  string
	CampaignNo string
	BenefitNo  string
	CodeLength int
	Count      int
}

type CouponBatchFile struct {
	BatchNo     string
	FileName    string
	Body        []byte
	Count       int
	GeneratedAt time.Time
}

type CouponBatchService interface {
	Generate(ctx context.Context, input CouponBatchGenerateInput) (*CouponBatchFile, error)
}

type couponBatchServiceImpl struct {
	database   gdb.DB
	audit      idccap.BusinessAuditWriter
	authorizer ManagementActionAuthorizer
	key        []byte
}

var _ CouponBatchService = (*couponBatchServiceImpl)(nil)

func NewCouponBatchService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementActionAuthorizer,
	key []byte,
) (CouponBatchService, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(auditWriter) ||
		isNilMarketingManagementDependency(authorizer) || len(key) < 32 {
		return nil, gerror.New("coupon batch generation requires database, audit, authorization, and installation key")
	}
	return &couponBatchServiceImpl{
		database: database, audit: auditWriter, authorizer: authorizer, key: append([]byte(nil), key...),
	}, nil
}

func (s *couponBatchServiceImpl) Generate(
	ctx context.Context,
	input CouponBatchGenerateInput,
) (*CouponBatchFile, error) {
	actorUserID, err := s.authorizer.AuthorizeCouponCodeGenerate(ctx)
	if err != nil {
		return nil, err
	}
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	input.BenefitNo = strings.TrimSpace(input.BenefitNo)
	if actorUserID <= 0 || !validBusinessReference(input.RequestNo) ||
		!identifierPattern.MatchString(input.CampaignNo) || !identifierPattern.MatchString(input.BenefitNo) ||
		input.CodeLength < 8 || input.CodeLength > 24 || input.Count < 1 || input.Count > maximumCouponBatchSize {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	requestDigest := marketingFactDigest(
		"coupon-batch", input.RequestNo, input.CampaignNo, input.BenefitNo,
		fmt.Sprintf("%d", input.CodeLength), fmt.Sprintf("%d", input.Count),
	)
	batchNo := "mcb_" + requestDigest[:24]
	codes, err := generateUniqueCouponCodes(input.Count, input.CodeLength)
	if err != nil {
		return nil, gerror.Wrap(err, "generate coupon codes")
	}
	var rule *managedAssetRuleRow
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, "marketing.coupon.batch:"+input.RequestNo); lockErr != nil {
			return gerror.Wrap(lockErr, "lock coupon batch request")
		}
		existing, loadErr := s.database.GetValue(txCtx, `SELECT count(*) FROM plugin_idc_core_marketing_coupon_batch_request WHERE tenant_id=0 AND request_no=?`, input.RequestNo)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "load coupon batch request")
		}
		if existing.Int() != 0 {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		var rows []managedAssetRuleRow
		if loadErr := s.database.GetScan(txCtx, &rows, `
SELECT c.campaign_no, c.campaign_type_code, c.target_segment_no, c.target_snapshot_no,
       c.priority, c.stack_policy_code, c.stack_group_code, c.version AS campaign_version,
       c.global_quota_limit, c.per_customer_quota_limit, c.ends_at, c.is_newcomer_exclusive,
       b.benefit_no, b.benefit_type_code, b.currency_code, b.amount_minor,
       b.rate_basis_points, b.maximum_discount_minor, b.minimum_spend_minor,
       b.purchase_phase_code, b.cycle_scope_code, b.cycle_count,
       b.version AS benefit_version, CURRENT_TIMESTAMP AS database_now
FROM plugin_idc_core_marketing_campaign c
JOIN plugin_idc_core_marketing_benefit_rule b
  ON b.tenant_id=c.tenant_id AND b.campaign_no=c.campaign_no
WHERE c.tenant_id=0 AND c.campaign_no=? AND b.benefit_no=?
  AND c.status_code='active' AND CURRENT_TIMESTAMP>=c.starts_at AND CURRENT_TIMESTAMP<c.ends_at
  AND c.campaign_type_code='coupon' AND b.status_code='active'
  AND b.benefit_type_code IN ('fixed_discount','percentage_discount')
FOR SHARE OF c, b`, input.CampaignNo, input.BenefitNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load coupon batch rule")
		}
		if len(rows) != 1 || rows[0].CurrencyCode == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		rule = &rows[0]
		if rule.GlobalQuotaLimit != nil {
			issued, countErr := s.database.GetValue(txCtx, `SELECT count(*) FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id=0 AND campaign_no=?`, input.CampaignNo)
			if countErr != nil {
				return gerror.Wrap(countErr, "count campaign coupon assets")
			}
			if issued.Int64()+int64(input.Count) > *rule.GlobalQuotaLimit {
				return bizerr.NewCode(CodeManagementActionConflict)
			}
		}
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_marketing_coupon_batch_request
 (tenant_id,batch_no,request_no,request_digest,campaign_no,benefit_no,code_length,generated_count,generated_by,generated_at)
VALUES (0,?,?,?,?,?,?,?,?,?)`, batchNo, input.RequestNo, requestDigest, input.CampaignNo, input.BenefitNo,
			input.CodeLength, input.Count, actorUserID, rule.DatabaseNow); insertErr != nil {
			return gerror.Wrap(insertErr, "insert coupon batch request")
		}
		benefitSnapshot, digestErr := benefitDigest(rule.candidate())
		if digestErr != nil {
			return gerror.Wrap(digestErr, "digest coupon batch benefit")
		}
		values := make([]string, 0, input.Count)
		args := make([]any, 0, input.Count*13)
		for index, code := range codes {
			assetDigest := sha256.Sum256([]byte(fmt.Sprintf("marketing.coupon.batch.asset\x00%s\x00%d", batchNo, index)))
			assetNo := "mcpa_" + hex.EncodeToString(assetDigest[:12])
			values = append(values, "(0,?,?,?,?,'coupon',NULL,?,?,?,?,?,1,0,0,?,?,'issued',NULL,?)")
			args = append(args, assetNo, input.CampaignNo, input.BenefitNo, rule.BenefitType,
				s.couponDigest(code), benefitSnapshot, *rule.CurrencyCode, rule.AmountMinor, rule.RateBasisPoints,
				rule.DatabaseNow, rule.EndsAt, batchNo)
		}
		query := `INSERT INTO plugin_idc_core_marketing_promotion_asset
 (tenant_id,asset_no,campaign_no,benefit_no,benefit_type_code,asset_type_code,customer_no,
  claim_code_digest,benefit_snapshot_digest,currency_code,face_amount_minor,rate_basis_points,
  usage_limit,reserved_uses,consumed_uses,valid_from,valid_until,status_code,claimed_at,coupon_batch_no)
VALUES ` + strings.Join(values, ",")
		if _, insertErr := s.database.Exec(txCtx, query, args...); insertErr != nil {
			return gerror.Wrap(insertErr, "insert coupon batch assets")
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "marketing.coupon.batch.generate", IdempotencyKey: input.RequestNo,
			OperationCode: "marketing.coupon.batch.generate", TargetType: "marketing_coupon_batch", TargetNo: batchNo,
			ResultCode: "succeeded", ReasonCode: "campaign_fulfillment", RiskCode: "high",
			RequiredPermission: permissionMarketingCouponGenerate, CorrelationNo: input.CampaignNo,
			AfterSnapshot: map[string]any{
				"batch_no": batchNo, "campaign_no": input.CampaignNo, "benefit_no": input.BenefitNo,
				"generated_count": input.Count, "code_length": input.CodeLength,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	body, err := buildCouponBatchWorkbook(batchNo, input, codes, rule.DatabaseNow, rule.EndsAt)
	if err != nil {
		return nil, err
	}
	return &CouponBatchFile{
		BatchNo: batchNo, FileName: "coupon-codes-" + batchNo + ".xlsx", Body: body,
		Count: input.Count, GeneratedAt: rule.DatabaseNow,
	}, nil
}

func (s *couponBatchServiceImpl) couponDigest(code string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("idc/marketing/coupon-code/v1\x00"))
	_, _ = mac.Write([]byte(code))
	return mac.Sum(nil)
}

func generateUniqueCouponCodes(count, length int) ([]string, error) {
	result := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	buffer := make([]byte, length)
	for len(result) < count {
		if _, err := rand.Read(buffer); err != nil {
			return nil, err
		}
		code := make([]byte, length)
		for index := range buffer {
			code[index] = couponAlphabet[int(buffer[index])%len(couponAlphabet)]
		}
		value := string(code)
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func buildCouponBatchWorkbook(
	batchNo string,
	input CouponBatchGenerateInput,
	codes []string,
	generatedAt time.Time,
	validUntil time.Time,
) (body []byte, resultErr error) {
	file := excelize.NewFile()
	defer func() {
		if closeErr := file.Close(); resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
	}()
	const sheet = "Coupon Codes"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, gerror.Wrap(err, "name coupon workbook sheet")
	}
	headers := []string{"Batch No", "Campaign No", "Benefit No", "Coupon Code", "Generated At", "Valid Until"}
	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return nil, gerror.Wrap(err, "write coupon workbook header")
		}
	}
	for index, code := range codes {
		row := index + 2
		values := []any{batchNo, input.CampaignNo, input.BenefitNo, code, generatedAt.UTC().Format(time.RFC3339), validUntil.UTC().Format(time.RFC3339)}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, row)
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return nil, gerror.Wrap(err, "write coupon workbook row")
			}
		}
	}
	if err := file.SetColWidth(sheet, "A", "F", 24); err != nil {
		return nil, gerror.Wrap(err, "size coupon workbook columns")
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, gerror.Wrap(err, "encode coupon workbook")
	}
	return buffer.Bytes(), nil
}
