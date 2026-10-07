// This file implements customer-owned contract reads, short-lived signing links, and authoritative rechecks.

package electroniccontractpreparation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"
	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	defaultCustomerContractLimit = 20
	maximumCustomerContractLimit = 50
)

var customerGatewayRoutePattern = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)

type customerContractCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

type customerContractRow struct {
	ID               int64     `orm:"id"`
	ContractNo       string    `orm:"contract_no"`
	OrderNo          string    `orm:"order_no"`
	CustomerNo       string    `orm:"customer_no"`
	TemplateName     string    `orm:"template_name"`
	LanguageCode     string    `orm:"language_code"`
	AmountMinor      int64     `orm:"amount_minor"`
	CurrencyCode     string    `orm:"currency_code"`
	StatusCode       string    `orm:"status_code"`
	CreatedAt        time.Time `orm:"created_at"`
	SigningExpiresAt time.Time `orm:"signing_expires_at"`
	Version          int64     `orm:"version"`
	FlowNo           string    `orm:"flow_no"`
	FlowStatusCode   string    `orm:"flow_status_code"`
	GatewayRouteNo   string    `orm:"gateway_route_no"`
	DatabaseNow      time.Time `orm:"database_now"`
}

const customerContractProjection = `
c.id, c.contract_no, c.order_no, c.customer_no,
tv.name AS template_name, tv.language_code,
c.amount_minor, c.currency_code, c.status_code, c.created_at,
COALESCE(flow.expires_at, c.signing_expires_at) AS signing_expires_at, c.version,
COALESCE(flow.flow_no, '') AS flow_no,
COALESCE(flow.status_code, '') AS flow_status_code,
COALESCE(flow.gateway_route_no, '') AS gateway_route_no,
CURRENT_TIMESTAMP AS database_now`

func (s *serviceImpl) ListCurrentContracts(
	ctx context.Context,
	query CustomerContractQuery,
) (*CustomerContractPage, error) {
	if s == nil || s.customers == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	limit := query.Limit
	if limit == 0 {
		limit = defaultCustomerContractLimit
	}
	if limit < 1 || limit > maximumCustomerContractLimit {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if subject == nil || !validBusinessNo(subject.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	cursor, err := decodeCustomerContractCursor(query.Cursor)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	arguments := []any{idccap.DefaultTenantID, subject.CustomerNo}
	cursorPredicate := ""
	if cursor != nil {
		cursorPredicate = " AND (c.created_at, c.id) < (?, ?)"
		arguments = append(arguments, cursor.CreatedAt.UTC(), cursor.ID)
	}
	arguments = append(arguments, limit+1)
	var rows []customerContractRow
	err = s.database.GetScan(ctx, &rows, `SELECT `+customerContractProjection+`
FROM plugin_idc_compliance_contract AS c
JOIN plugin_idc_compliance_contract_template_version AS tv
  ON tv.tenant_id = c.tenant_id AND tv.template_no = c.template_no
 AND tv.version_no = c.template_version
LEFT JOIN LATERAL (
    SELECT f.flow_no, f.status_code, f.gateway_route_no, f.expires_at
    FROM plugin_idc_compliance_contract_flow AS f
    WHERE f.tenant_id = c.tenant_id AND f.contract_no = c.contract_no
    ORDER BY f.sequence_no DESC, f.id DESC
    LIMIT 1
) AS flow ON TRUE
WHERE c.tenant_id = ? AND c.customer_no = ?`+cursorPredicate+`
ORDER BY c.created_at DESC, c.id DESC
LIMIT ?`, arguments...)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := &CustomerContractPage{Items: make([]CustomerContract, 0, len(rows))}
	for index := range rows {
		item, projectErr := projectCustomerContract(&rows[index])
		if projectErr != nil {
			return nil, bizerr.WrapCode(projectErr, CodeUnavailable)
		}
		page.Items = append(page.Items, *item)
	}
	if hasMore && len(rows) > 0 {
		page.NextCursor, err = encodeCustomerContractCursor(customerContractCursor{
			CreatedAt: rows[len(rows)-1].CreatedAt, ID: rows[len(rows)-1].ID,
		})
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
	}
	return page, nil
}

func (s *serviceImpl) CurrentOrderContract(
	ctx context.Context,
	orderNo string,
) (*CustomerContract, error) {
	orderNo = strings.TrimSpace(orderNo)
	if !validBusinessNo(orderNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadCurrentCustomerContract(ctx, "c.order_no = ?", orderNo)
	if err != nil {
		return nil, err
	}
	return projectCustomerContract(row)
}

func (s *serviceImpl) CreateCurrentSigningLink(
	ctx context.Context,
	contractNo string,
) (*CustomerSigningLink, error) {
	contractNo = strings.TrimSpace(contractNo)
	if !validBusinessNo(contractNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadCurrentCustomerContract(ctx, "c.contract_no = ?", contractNo)
	if err != nil {
		return nil, err
	}
	contract, err := projectCustomerContract(row)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if !contract.CanOpenSigning || s.gateway == nil || !validSigningReturnURL(s.signingReturnURL) {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	stored, err := s.loadByContractNo(ctx, contractNo)
	if err != nil || stored.CustomerNo != row.CustomerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	handle, err := secretcap.NewHandle(stored.SnapshotKeyRef)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	expected := snapshotDigests{
		Variables: stored.VariableSnapshotSHA256, Signers: stored.SignerSnapshotSHA256,
		Order: stored.OrderSnapshotSHA256,
	}
	var input processauth.TencentContractSigningLinkRequest
	err = s.withSnapshotBundle(
		ctx, handle, "compliance.contract-customer-signing-link", &expected,
		func(bundle *snapshotBundle) error {
			if validateBundleRow(bundle, stored) != nil || bundle.Order.Customer.CustomerNo != row.CustomerNo {
				return gerror.New("customer contract snapshot is unavailable")
			}
			var recipient *providerRecipientSnapshot
			for index := range bundle.ProviderLaunch.Recipients {
				candidate := &bundle.ProviderLaunch.Recipients[index]
				if candidate.PartyCode == providerPartyCustomer {
					if recipient != nil {
						return gerror.New("customer contract recipient is ambiguous")
					}
					recipient = candidate
				}
			}
			if recipient == nil {
				return gerror.New("customer contract recipient is unavailable")
			}
			input = processauth.TencentContractSigningLinkRequest{
				RouteNo: row.GatewayRouteNo, JumpURL: s.signingReturnURL,
				Approver: processauth.TencentContractApprover{
					SequenceNo: recipient.SequenceNo, RecipientRef: recipient.RecipientRef,
					PartyCode: providerPartyCustomer, SubjectTypeCode: string(bundle.CustomerSigner.CustomerType),
					Name: string(bundle.CustomerSigner.Name), Mobile: string(bundle.CustomerSigner.Phone),
				},
			}
			if bundle.CustomerSigner.CustomerType == contractcap.CustomerTypeEnterprise {
				input.Approver.OrganizationName = string(bundle.Order.Customer.LegalName)
			}
			return nil
		},
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	response, err := s.gateway.CreateTencentContractSigningLink(ctx, input)
	input.Approver.Name = ""
	input.Approver.Mobile = ""
	input.Approver.OrganizationName = ""
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	expiresAt := time.Unix(response.ExpiresUnix, 0).UTC()
	if !validHTTPSURL(response.URL) || !expiresAt.After(row.DatabaseNow) ||
		expiresAt.After(row.DatabaseNow.Add(31*time.Minute)) {
		return nil, bizerr.NewCode(CodeProviderUnavailable)
	}
	return &CustomerSigningLink{Contract: *contract, URL: response.URL, ExpiresAt: expiresAt}, nil
}

func (s *serviceImpl) RecheckCurrentContract(
	ctx context.Context,
	contractNo string,
) (*CustomerContract, error) {
	contractNo = strings.TrimSpace(contractNo)
	if !validBusinessNo(contractNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadCurrentCustomerContract(ctx, "c.contract_no = ?", contractNo)
	if err != nil {
		return nil, err
	}
	contract, err := projectCustomerContract(row)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if !contract.CanRecheck || s.gateway == nil {
		return nil, bizerr.NewCode(CodeNotSignable)
	}
	response, err := s.gateway.RecheckTencentContractFlow(
		ctx, processauth.TencentContractFlowRecheckRequest{RouteNo: row.GatewayRouteNo},
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	if response.RouteNo != row.GatewayRouteNo || response.RouteVersion < 1 {
		return nil, bizerr.NewCode(CodeProviderUnavailable)
	}
	if err = s.convergeCustomerContractRecheck(ctx, row, response); err != nil {
		return nil, err
	}
	updated, err := s.loadCurrentCustomerContract(ctx, "c.contract_no = ?", contractNo)
	if err != nil {
		return nil, err
	}
	return projectCustomerContract(updated)
}

func (s *serviceImpl) loadCurrentCustomerContract(
	ctx context.Context,
	predicate string,
	value string,
) (*customerContractRow, error) {
	if s == nil || s.customers == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if subject == nil || !validBusinessNo(subject.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var rows []customerContractRow
	err = s.database.GetScan(ctx, &rows, `SELECT `+customerContractProjection+`
FROM plugin_idc_compliance_contract AS c
JOIN plugin_idc_compliance_contract_template_version AS tv
  ON tv.tenant_id = c.tenant_id AND tv.template_no = c.template_no
 AND tv.version_no = c.template_version
LEFT JOIN LATERAL (
    SELECT f.flow_no, f.status_code, f.gateway_route_no, f.expires_at
    FROM plugin_idc_compliance_contract_flow AS f
    WHERE f.tenant_id = c.tenant_id AND f.contract_no = c.contract_no
    ORDER BY f.sequence_no DESC, f.id DESC
    LIMIT 1
) AS flow ON TRUE
WHERE c.tenant_id = ? AND c.customer_no = ? AND `+predicate+`
  AND NOT EXISTS (
      SELECT 1 FROM plugin_idc_compliance_contract_replacement AS replacement
      WHERE replacement.tenant_id = c.tenant_id
        AND replacement.old_contract_no = c.contract_no
  )
LIMIT 2`, idccap.DefaultTenantID, subject.CustomerNo, value)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &rows[0], nil
}

func projectCustomerContract(row *customerContractRow) (*CustomerContract, error) {
	if row == nil || row.ID < 1 || !validBusinessNo(row.ContractNo, 64) ||
		!validBusinessNo(row.OrderNo, 64) || !validBusinessNo(row.CustomerNo, 40) ||
		strings.TrimSpace(row.TemplateName) == "" ||
		(row.LanguageCode != "zh-CN" && row.LanguageCode != "en-US") ||
		row.AmountMinor < 0 || row.CurrencyCode == "" || row.CreatedAt.IsZero() ||
		row.SigningExpiresAt.IsZero() || row.DatabaseNow.IsZero() || row.Version < 1 {
		return nil, gerror.New("customer contract projection is invalid")
	}
	status := "processing"
	switch row.StatusCode {
	case "completed":
		status = "completed"
	case "rejected", "expired", "revoked", "failed", "terminated":
		status = "failed"
	case "draft", "pending_launch", "signing", "reissuing":
	default:
		return nil, gerror.New("customer contract state is invalid")
	}
	hasRoute := customerGatewayRoutePattern.MatchString(row.GatewayRouteNo)
	canOpen := row.StatusCode == "signing" && row.FlowStatusCode == "signing" &&
		hasRoute && row.DatabaseNow.Before(row.SigningExpiresAt)
	canRecheck := row.StatusCode == "signing" &&
		(row.FlowStatusCode == "signing" || row.FlowStatusCode == "result_unknown") && hasRoute
	return &CustomerContract{
		ContractNo: row.ContractNo, OrderNo: row.OrderNo, TemplateName: row.TemplateName,
		LanguageCode: row.LanguageCode, AmountMinor: row.AmountMinor, CurrencyCode: row.CurrencyCode,
		StatusCode: status, CreatedAt: row.CreatedAt.UTC(), SigningExpiresAt: row.SigningExpiresAt.UTC(),
		CanOpenSigning: canOpen, CanRecheck: canRecheck, Version: row.Version,
	}, nil
}

func (s *serviceImpl) convergeCustomerContractRecheck(
	ctx context.Context,
	owned *customerContractRow,
	response processauth.TencentContractFlowRecheckResponse,
) error {
	if owned == nil || response.RouteNo != owned.GatewayRouteNo || s.lifecycle == nil {
		return bizerr.NewCode(CodeNotAccessible)
	}
	if response.DecisionCode == "query_failed" {
		return nil
	}
	supplierObservedAt := time.Unix(response.SupplierObservedUnix, 0).UTC()
	_, err := s.lifecycle.Apply(ctx, contractlifecycle.Observation{
		RouteNo: response.RouteNo, RouteVersion: response.RouteVersion,
		ProviderStatus: response.ProviderStatus, SourceKind: "active_query",
		GatewayDecisionCode: response.DecisionCode,
		SupplierObservedAt:  supplierObservedAt, ObservedAt: supplierObservedAt,
	}, s.runtime.Audit)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}

func encodeCustomerContractCursor(cursor customerContractCursor) (string, error) {
	if cursor.ID < 1 || cursor.CreatedAt.IsZero() {
		return "", gerror.New("customer contract cursor is invalid")
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCustomerContractCursor(value string) (*customerContractCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > 256 {
		return nil, gerror.New("customer contract cursor is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, gerror.New("customer contract cursor is invalid")
	}
	var cursor customerContractCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.ID < 1 || cursor.CreatedAt.IsZero() {
		return nil, gerror.New("customer contract cursor is invalid")
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	return &cursor, nil
}

func validSigningReturnURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.Path != "/account/contracts" || parsed.RawQuery != "signing_return=1" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	hostname := parsed.Hostname()
	loopback := strings.EqualFold(hostname, "localhost")
	if ip := net.ParseIP(hostname); ip != nil {
		loopback = ip.IsLoopback()
	}
	return parsed.Scheme == "http" && loopback
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.Hostname() != "" &&
		parsed.User == nil && parsed.Fragment == "" && !strings.ContainsAny(value, "\r\n\t\x00")
}
