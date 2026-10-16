// This file builds frozen Tencent requests and reserves guarded contract launches.

package electroniccontractpreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"golang.org/x/text/currency"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

var tencentMobilePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

type contractFlowRow struct {
	FlowNo                string     `orm:"flow_no"`
	ContractNo            string     `orm:"contract_no"`
	IdempotencySHA256     string     `orm:"idempotency_sha256"`
	RequestSnapshotRef    string     `orm:"request_snapshot_ref"`
	RequestSnapshotSHA256 string     `orm:"request_snapshot_sha256"`
	GatewayRouteNo        *string    `orm:"gateway_route_no"`
	StatusCode            string     `orm:"status_code"`
	ExpiresAt             *time.Time `orm:"expires_at"`
	Version               int64      `orm:"version"`
}

type serviceTablePayload struct {
	Headers  []serviceTableHeader `json:"headers"`
	RowCount int                  `json:"rowCount"`
	Body     serviceTableBody     `json:"body"`
}

type serviceTableHeader struct {
	Content string `json:"content"`
}

type serviceTableBody struct {
	Cells []serviceTableCell `json:"cells"`
}

type serviceTableCell struct {
	RowStart    int    `json:"rowStart"`
	RowEnd      int    `json:"rowEnd"`
	ColumnStart int    `json:"columnStart"`
	ColumnEnd   int    `json:"columnEnd"`
	Content     string `json:"content"`
}

func (s *serviceImpl) SubmitSigning(
	ctx context.Context,
	input SubmitSigningInput,
) (*SubmittedSigning, error) {
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if s == nil || s.gateway == nil || ctx == nil || ctx.Err() != nil ||
		!validBusinessNo(input.ContractNo, 64) || input.ExpectedVersion < 1 ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, gerror.New("electronic contract signing submission input is invalid")
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractCreate, TargetNo: input.ContractNo,
	}); err != nil {
		return nil, err
	}
	row, err := s.loadByContractNo(ctx, input.ContractNo)
	if err != nil || validateStoredRow(row) != nil {
		return nil, gerror.New("prepared contract is unavailable for signing submission")
	}
	var gatewayRequest processauth.TencentContractFlowRequest
	handle, err := secretcap.NewHandle(row.SnapshotKeyRef)
	if err != nil {
		return nil, err
	}
	expected := snapshotDigests{
		Variables: row.VariableSnapshotSHA256, Signers: row.SignerSnapshotSHA256,
		Order: row.OrderSnapshotSHA256,
	}
	err = s.withSnapshotBundle(ctx, handle, "compliance.contract-submit", &expected,
		func(bundle *snapshotBundle) error {
			if validateErr := validateBundleRow(bundle, row); validateErr != nil {
				return validateErr
			}
			request, requestErr := s.buildContractFlowRequest(bundle)
			if requestErr != nil {
				return requestErr
			}
			gatewayRequest = request
			return nil
		})
	if err != nil {
		return nil, err
	}
	requestBytes, err := json.Marshal(gatewayRequest)
	if err != nil || len(requestBytes) == 0 {
		clear(requestBytes)
		return nil, gerror.New("electronic contract gateway request is invalid")
	}
	requestDigest := sha256.Sum256(requestBytes)
	clear(requestBytes)
	requestSHA256 := hex.EncodeToString(requestDigest[:])
	idempotencyDigest := digestText("idc/contract-submit-idempotency/v1\x00", input.IdempotencyKey)
	flow, contractVersion, launchRequired, err := s.reserveContractFlow(
		ctx, row, input.ExpectedVersion, idempotencyDigest, requestSHA256,
	)
	if err != nil {
		return nil, err
	}
	if !launchRequired {
		return &SubmittedSigning{
			ContractNo: input.ContractNo, FlowNo: flow.FlowNo,
			StatusCode: flow.StatusCode, Version: contractVersion, Reused: true,
		}, nil
	}
	result, gatewayErr := s.gateway.CreateTencentContractFlow(
		ctx, gatewayRequest, "contract-submit:"+idempotencyDigest,
	)
	resultUnknown := false
	if unknown, ok := gatewayErr.(interface{ IsResultUnknown() bool }); ok {
		resultUnknown = unknown.IsResultUnknown()
	}
	statusCode, finalVersion, finishErr := s.finishContractFlow(
		ctx, flow, requestSHA256, result.RouteNo, gatewayErr, resultUnknown,
	)
	if finishErr != nil {
		return nil, finishErr
	}
	if gatewayErr != nil {
		return nil, gatewayErr
	}
	return &SubmittedSigning{
		ContractNo: input.ContractNo, FlowNo: flow.FlowNo,
		StatusCode: statusCode, Version: finalVersion, Reused: result.Recovered,
	}, nil
}

func (s *serviceImpl) appendSigningSubmissionAudit(
	ctx context.Context,
	flow *contractFlowRow,
	statusCode string,
) error {
	if s == nil || flow == nil || flow.FlowNo == "" || flow.ContractNo == "" ||
		(statusCode != "submitting" && statusCode != "signing" &&
			statusCode != "result_unknown" && statusCode != "failed") {
		return gerror.New("contract signing audit input is invalid")
	}
	reasonCode := "provider_submission_reserved"
	if statusCode == "signing" {
		reasonCode = "provider_flow_created"
	} else if statusCode == "result_unknown" {
		reasonCode = "provider_result_unknown"
	} else if statusCode == "failed" {
		reasonCode = "provider_flow_failed"
	}
	beforeState := "not_created"
	if statusCode != "submitting" {
		beforeState = flow.StatusCode
	}
	if err := s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"flow_state": beforeState},
		IdempotencyScope: "compliance.contract.submit." + statusCode,
		IdempotencyKey:   flow.IdempotencySHA256,
		OperationCode:    "compliance.contract.submit",
		TargetType:       "contract", TargetNo: flow.ContractNo,
		ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: reasonCode, RiskCode: "high",
		CorrelationNo: flow.FlowNo,
		AfterSnapshot: map[string]any{
			"state": statusCode, "flow_no": flow.FlowNo,
		},
	}); err != nil {
		return gerror.Wrap(err, "append contract signing submission audit")
	}
	return nil
}

func (s *serviceImpl) reserveContractFlow(
	ctx context.Context,
	loaded *contractRow,
	expectedVersion int64,
	idempotencyDigest string,
	requestDigest string,
) (*contractFlowRow, int64, bool, error) {
	var flow *contractFlowRow
	contractVersion := int64(0)
	launchRequired := false
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, key := range []string{"contract-flow:" + loaded.ContractNo, "contract-flow-idem:" + idempotencyDigest} {
			if _, lockErr := tx.GetValue("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); lockErr != nil {
				return gerror.Wrap(lockErr, "lock electronic contract flow")
			}
		}
		var contracts []*contractRow
		if scanErr := tx.Ctx(ctx).GetScan(&contracts, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?
FOR UPDATE`, idccap.DefaultTenantID, loaded.ContractNo); scanErr != nil {
			return gerror.Wrap(scanErr, "lock electronic contract")
		}
		if len(contracts) != 1 || validateStoredRow(contracts[0]) != nil {
			return gerror.New("prepared contract is unavailable for signing submission")
		}
		contract := contracts[0]
		contractVersion = contract.Version
		var flows []*contractFlowRow
		if scanErr := tx.Ctx(ctx).GetScan(&flows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND (idempotency_sha256 = ? OR (contract_no = ? AND purpose_code = 'issue'))
ORDER BY CASE WHEN idempotency_sha256 = ? THEN 0 ELSE 1 END, id ASC
LIMIT 2
FOR UPDATE`, idccap.DefaultTenantID, idempotencyDigest, loaded.ContractNo, idempotencyDigest); scanErr != nil {
			return gerror.Wrap(scanErr, "load electronic contract flow")
		}
		if len(flows) > 1 {
			return gerror.New("contract signing submission conflicts with an existing flow")
		}
		if len(flows) == 1 {
			flow = flows[0]
			if flow == nil || flow.ContractNo != loaded.ContractNo ||
				flow.IdempotencySHA256 != idempotencyDigest ||
				flow.RequestSnapshotRef != contract.SnapshotKeyRef ||
				flow.RequestSnapshotSHA256 != requestDigest {
				return gerror.New("contract signing idempotency key conflicts with another request")
			}
			switch flow.StatusCode {
			case "signing", "result_unknown":
				return nil
			case "queued", "submitting":
				return gerror.New("contract signing submission is already in progress")
			default:
				return gerror.New("existing contract signing flow cannot be resubmitted")
			}
		}
		databaseNow, timeErr := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if timeErr != nil {
			return gerror.Wrap(timeErr, "load contract submission time")
		}
		now := databaseNow.Time().UTC()
		if contract.StatusCode != "pending_launch" || contract.Version != expectedVersion ||
			!now.Before(contract.SigningExpiresAt) {
			return gerror.New("contract preview is no longer eligible for signing submission")
		}
		flowNo, numberErr := newFlowNo(s.random)
		if numberErr != nil {
			return numberErr
		}
		if _, insertErr := tx.Exec(`
INSERT INTO plugin_idc_compliance_contract_flow
    (tenant_id, flow_no, contract_no, sequence_no, purpose_code, provider_code,
     idempotency_sha256, request_snapshot_ref, request_snapshot_sha256,
     status_code, available_at, expires_at, version, created_at, updated_at)
VALUES (?, ?, ?, 1, 'issue', ?, ?, ?, ?, 'submitting', ?, ?, 1, ?, ?)`,
			idccap.DefaultTenantID, flowNo, contract.ContractNo, ProviderTencentESSBasic,
			idempotencyDigest, contract.SnapshotKeyRef, requestDigest,
			now, contract.SigningExpiresAt, now, now); insertErr != nil {
			return gerror.Wrap(insertErr, "reserve electronic contract flow")
		}
		flow = &contractFlowRow{
			FlowNo: flowNo, ContractNo: contract.ContractNo, IdempotencySHA256: idempotencyDigest,
			RequestSnapshotRef: contract.SnapshotKeyRef, RequestSnapshotSHA256: requestDigest,
			StatusCode: "submitting", ExpiresAt: &contract.SigningExpiresAt, Version: 1,
		}
		if auditErr := s.appendSigningSubmissionAudit(ctx, flow, flow.StatusCode); auditErr != nil {
			return auditErr
		}
		launchRequired = true
		return nil
	})
	if err != nil {
		return nil, 0, false, err
	}
	return flow, contractVersion, launchRequired, nil
}

func (s *serviceImpl) finishContractFlow(
	ctx context.Context,
	reserved *contractFlowRow,
	requestDigest string,
	gatewayRouteNo string,
	gatewayErr error,
	resultUnknown bool,
) (string, int64, error) {
	statusCode := "signing"
	contractStatus := "signing"
	failureCode := ""
	if gatewayErr != nil {
		failureCode = boundedFailureCode(gatewayErr.Error())
		if resultUnknown {
			statusCode = "result_unknown"
		} else {
			statusCode = "failed"
			contractStatus = "failed"
		}
	}
	version := int64(0)
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var flows []*contractFlowRow
		if scanErr := tx.Ctx(ctx).GetScan(&flows, `
SELECT flow_no, contract_no, idempotency_sha256, request_snapshot_ref,
       request_snapshot_sha256, gateway_route_no, status_code, expires_at, version
FROM plugin_idc_compliance_contract_flow
WHERE tenant_id = ? AND flow_no = ?
FOR UPDATE`, idccap.DefaultTenantID, reserved.FlowNo); scanErr != nil {
			return gerror.Wrap(scanErr, "lock submitted electronic contract flow")
		}
		if len(flows) != 1 || flows[0] == nil || flows[0].ContractNo != reserved.ContractNo ||
			flows[0].IdempotencySHA256 != reserved.IdempotencySHA256 ||
			flows[0].RequestSnapshotSHA256 != requestDigest {
			return gerror.New("submitted electronic contract flow changed unexpectedly")
		}
		if flows[0].StatusCode == "signing" && gatewayErr == nil {
			value, loadErr := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, reserved.ContractNo)
			if loadErr != nil {
				return loadErr
			}
			version = value.Int64()
			return nil
		}
		if flows[0].StatusCode != "submitting" && flows[0].StatusCode != "queued" &&
			flows[0].StatusCode != "result_unknown" {
			return gerror.New("submitted electronic contract flow is not mutable")
		}
		databaseNow, timeErr := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if timeErr != nil {
			return timeErr
		}
		now := databaseNow.Time().UTC()
		var route any
		if gatewayRouteNo != "" {
			route = gatewayRouteNo
		}
		result, updateErr := tx.Exec(`
UPDATE plugin_idc_compliance_contract_flow
SET gateway_route_no = ?, status_code = ?, submitted_at = CASE WHEN ? IN ('signing','result_unknown') THEN COALESCE(submitted_at, ?) ELSE submitted_at END,
    failure_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND flow_no = ? AND version = ?`, route, statusCode, statusCode,
			now, failureCode, now, idccap.DefaultTenantID, reserved.FlowNo, flows[0].Version)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "finish electronic contract flow")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("electronic contract flow update conflicted")
		}
		contractResult, contractErr := tx.Exec(`
UPDATE plugin_idc_compliance_contract
SET status_code = ?, launched_at = CASE WHEN ? = 'signing' THEN COALESCE(launched_at, ?) ELSE launched_at END,
    version = version + 1, updated_at = ?
WHERE tenant_id = ? AND contract_no = ? AND status_code IN ('pending_launch','signing')`,
			contractStatus, contractStatus, now, now, idccap.DefaultTenantID, reserved.ContractNo)
		if contractErr != nil {
			return gerror.Wrap(contractErr, "finish submitted electronic contract")
		}
		if affected, affectedErr := contractResult.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("electronic contract state update conflicted")
		}
		value, loadErr := tx.GetValue(`SELECT version FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?`, idccap.DefaultTenantID, reserved.ContractNo)
		if loadErr != nil {
			return loadErr
		}
		version = value.Int64()
		return s.appendSigningSubmissionAudit(ctx, reserved, statusCode)
	})
	if err != nil {
		return "", 0, err
	}
	return statusCode, version, nil
}

func (s *serviceImpl) buildContractFlowRequest(bundle *snapshotBundle) (processauth.TencentContractFlowRequest, error) {
	if bundle == nil {
		return processauth.TencentContractFlowRequest{}, gerror.New("frozen contract launch snapshot is invalid")
	}
	return s.buildContractFlowRequestWithDeadline(bundle, bundle.SigningExpiresAt)
}

func (s *serviceImpl) buildContractFlowRequestWithDeadline(
	bundle *snapshotBundle,
	deadline time.Time,
) (processauth.TencentContractFlowRequest, error) {
	if bundle == nil || validateCustomerSignerSnapshot(&bundle.CustomerSigner, &bundle.Order) != nil ||
		validateProviderLaunchSnapshot(&bundle.ProviderLaunch, &bundle.Order) != nil || deadline.IsZero() {
		return processauth.TencentContractFlowRequest{}, gerror.New("frozen contract launch snapshot is invalid")
	}
	serviceTable, err := s.buildServiceTable(bundle)
	if err != nil {
		return processauth.TencentContractFlowRequest{}, err
	}
	values := map[string]string{
		"contract_no": string(bundle.ContractNo), "customer_name": string(bundle.Preview.CustomerName),
		"merchant_name": string(bundle.Preview.MerchantName), "order_no": bundle.Preview.OrderNo,
		"service_lines": serviceTable, "currency": bundle.Preview.CurrencyCode,
	}
	values["total_amount"], err = formatMinorAmount(bundle.Preview.CurrencyCode, bundle.Preview.AmountMinor)
	if err != nil {
		return processauth.TencentContractFlowRequest{}, err
	}
	request := processauth.TencentContractFlowRequest{
		ContractNo: bundle.ContractNo, TemplateRef: bundle.ProviderLaunch.TemplateRef,
		LanguageCode: bundle.ProviderLaunch.LanguageCode, DeadlineUnix: deadline.Unix(),
		FormFields: make([]processauth.TencentContractFormField, 0, len(bundle.ProviderLaunch.Fields)),
		Approvers:  make([]processauth.TencentContractApprover, 0, len(bundle.ProviderLaunch.Recipients)),
	}
	for _, field := range bundle.ProviderLaunch.Fields {
		if field.ValueCode == "" || field.ValueCode == providerValueSigningDate {
			continue
		}
		value, exists := values[field.ValueCode]
		if !exists || value == "" {
			return processauth.TencentContractFlowRequest{}, gerror.New("frozen contract field value is unavailable")
		}
		request.FormFields = append(request.FormFields, processauth.TencentContractFormField{
			ComponentName: field.Name, TypeCode: field.TypeCode, Value: value,
		})
	}
	mobile, err := normalizeTencentMobile(bundle.CustomerSigner.Phone)
	if err != nil {
		return processauth.TencentContractFlowRequest{}, err
	}
	customerCount := 0
	merchantCount := 0
	for _, recipient := range bundle.ProviderLaunch.Recipients {
		approver := processauth.TencentContractApprover{
			SequenceNo: recipient.SequenceNo, RecipientRef: recipient.RecipientRef,
			PartyCode: recipient.PartyCode,
		}
		switch recipient.PartyCode {
		case providerPartyCustomer:
			customerCount++
			approver.SubjectTypeCode = string(bundle.CustomerSigner.CustomerType)
			approver.Name = string(bundle.CustomerSigner.Name)
			approver.Mobile = mobile
			if bundle.CustomerSigner.CustomerType == "enterprise" {
				approver.OrganizationName = string(bundle.Order.Customer.LegalName)
			}
		case providerPartyMerchant:
			merchantCount++
			approver.SubjectTypeCode = "enterprise"
			approver.OrganizationName = string(bundle.Order.Merchant.LegalName)
		default:
			return processauth.TencentContractFlowRequest{}, gerror.New("frozen contract approver mapping is incomplete")
		}
		request.Approvers = append(request.Approvers, approver)
	}
	if customerCount != 1 || merchantCount != 1 || len(request.Approvers) != len(bundle.ProviderLaunch.Recipients) {
		return processauth.TencentContractFlowRequest{}, gerror.New("frozen contract approver mapping is ambiguous")
	}
	return request, nil
}

func (s *serviceImpl) buildServiceTable(bundle *snapshotBundle) (string, error) {
	languageCode := bundle.ProviderLaunch.LanguageCode
	serviceHeader, err := s.artifactTexts.text(languageCode, artifactTextPrefix+"serviceTable.headers.service")
	if err != nil {
		return "", err
	}
	termHeader, err := s.artifactTexts.text(languageCode, artifactTextPrefix+"serviceTable.headers.term")
	if err != nil {
		return "", err
	}
	quantityHeader, err := s.artifactTexts.text(languageCode, artifactTextPrefix+"serviceTable.headers.quantity")
	if err != nil {
		return "", err
	}
	amountHeader, err := s.artifactTexts.text(languageCode, artifactTextPrefix+"serviceTable.headers.lineAmount")
	if err != nil {
		return "", err
	}
	headers := []string{serviceHeader, termHeader, quantityHeader, amountHeader + " (" + bundle.Preview.CurrencyCode + ")"}
	table := serviceTablePayload{RowCount: len(bundle.Preview.Items)}
	table.Headers = make([]serviceTableHeader, len(headers))
	for index, header := range headers {
		table.Headers[index] = serviceTableHeader{Content: header}
	}
	table.Body.Cells = make([]serviceTableCell, 0, len(bundle.Preview.Items)*4)
	for rowIndex, item := range bundle.Preview.Items {
		amount, err := formatMinorAmount(bundle.Preview.CurrencyCode, item.AmountMinor)
		if err != nil {
			return "", err
		}
		term, err := s.localizedServiceTerm(
			bundle.ProviderLaunch.LanguageCode, item.PeriodUnitCode,
			item.PeriodCount, item.PurchaseCycles,
		)
		if err != nil {
			return "", err
		}
		contents := []string{item.ServiceName, term, strconv.Itoa(item.Quantity), amount}
		for columnIndex, content := range contents {
			if !validServiceTableText(content) {
				return "", gerror.New("contract service table content exceeds Tencent limits")
			}
			row := rowIndex + 1
			column := columnIndex + 1
			table.Body.Cells = append(table.Body.Cells, serviceTableCell{
				RowStart: row, RowEnd: row, ColumnStart: column, ColumnEnd: column, Content: content,
			})
		}
	}
	encoded, err := json.Marshal(table)
	if err != nil || len(encoded) == 0 || len(encoded) > 256<<10 {
		clear(encoded)
		return "", gerror.New("contract service table is too large")
	}
	return string(encoded), nil
}

func formatMinorAmount(currencyCode string, amountMinor int64) (string, error) {
	if amountMinor < 0 {
		return "", gerror.New("contract amount is invalid")
	}
	unit, err := currency.ParseISO(currencyCode)
	if err != nil {
		return "", gerror.New("contract currency is unsupported")
	}
	scale, increment := currency.Standard.Rounding(unit)
	if scale < 0 || scale > 6 || increment != 1 {
		return "", gerror.New("contract currency rounding is unsupported")
	}
	divisor := int64(1)
	for range scale {
		divisor *= 10
	}
	if scale == 0 {
		return strconv.FormatInt(amountMinor, 10), nil
	}
	return fmt.Sprintf("%d.%0*d", amountMinor/divisor, scale, amountMinor%divisor), nil
}

func (s *serviceImpl) localizedServiceTerm(languageCode, unitCode string, periodCount, purchaseCycles int) (string, error) {
	if periodCount < 1 || purchaseCycles < 1 || periodCount > int(^uint(0)>>1)/purchaseCycles {
		return "", gerror.New("contract service term is invalid")
	}
	count := periodCount * purchaseCycles
	if languageCode != artifactLocaleChinese && languageCode != artifactLocaleEnglish {
		return "", gerror.New("contract service term language is unsupported")
	}
	switch unitCode {
	case "once", "hour", "day", "month", "year":
	default:
		return "", gerror.New("contract service term unit is unsupported")
	}
	form := "other"
	if count == 1 {
		form = "one"
	}
	label, err := s.artifactTexts.text(languageCode, artifactTextPrefix+"serviceTerm."+unitCode+"."+form)
	if err != nil {
		return "", err
	}
	separator := " "
	if languageCode == artifactLocaleChinese {
		separator = ""
	}
	return strconv.Itoa(count) + separator + label, nil
}

func normalizeTencentMobile(value []byte) (string, error) {
	mobile := strings.TrimSpace(string(value))
	mobile = strings.TrimPrefix(mobile, "+86")
	if !tencentMobilePattern.MatchString(mobile) {
		return "", gerror.New("verified contract signer mobile is unsupported by Tencent")
	}
	return mobile, nil
}

func validServiceTableText(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= 100 && !strings.ContainsAny(value, "\r\n\t\x00")
}

func newFlowNo(random io.Reader) (string, error) {
	value := make([]byte, 16)
	if random == nil {
		return "", gerror.New("contract flow random source is unavailable")
	}
	if _, err := io.ReadFull(random, value); err != nil {
		clear(value)
		return "", gerror.Wrap(err, "generate contract flow number")
	}
	defer clear(value)
	return "CFW_" + strings.ToUpper(hex.EncodeToString(value)), nil
}

func newReplacementNo(random io.Reader) (string, error) {
	value := make([]byte, 16)
	if random == nil {
		return "", gerror.New("contract replacement random source is unavailable")
	}
	if _, err := io.ReadFull(random, value); err != nil {
		clear(value)
		return "", gerror.Wrap(err, "generate contract replacement number")
	}
	defer clear(value)
	return "CRP_" + strings.ToUpper(hex.EncodeToString(value)), nil
}

func boundedFailureCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\r\n\t\x00") {
		return "contract_flow_failed"
	}
	return value
}
