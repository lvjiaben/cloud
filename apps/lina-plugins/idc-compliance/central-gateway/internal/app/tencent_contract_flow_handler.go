// This file serves guarded local Tencent contract flow, signing-link, and recheck routes.

package app

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/tencentess"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const tencentContractOperationScope = tencentess.OperationFlowCreate

var (
	contractFlowNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	tencentRefPattern     = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)
	mobilePattern         = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	gatewayRoutePattern   = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
)

type dynamicTablePayload struct {
	Headers  []dynamicTableHeader `json:"headers"`
	RowCount int                  `json:"rowCount"`
	Body     dynamicTableBody     `json:"body"`
}

type dynamicTableHeader struct {
	Content string `json:"content"`
}

type dynamicTableBody struct {
	Cells []dynamicTableCell `json:"cells"`
}

type dynamicTableCell struct {
	RowStart    int    `json:"rowStart"`
	RowEnd      int    `json:"rowEnd"`
	ColumnStart int    `json:"columnStart"`
	ColumnEnd   int    `json:"columnEnd"`
	Content     string `json:"content"`
}

func localTencentContractFlowHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentContractFlowsPath ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
		if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
			clearRuntimeBytes(body)
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		defer clearRuntimeBytes(body)
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, admission, _, err := runtime.authenticateRequest(
			request.Context(), processauth.TencentContractFlowCreateScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeJSON, body, credentials, true,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		var input processauth.TencentContractFlowRequest
		if err = decodeTencentOnboardingJSON(body, &input); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		flow, err := buildTencentContractFlow(input, time.Now().UTC())
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		result, err := runtime.tencentESS.CreateFlow(request.Context(), admission.requestNo, flow, false)
		if err != nil {
			writeTencentContractFlowError(writer, err, result.RouteNo)
			return
		}
		if result.RouteNo == "" {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_result_unknown"})
			return
		}
		writeJSON(writer, http.StatusOK, processauth.TencentContractFlowResponse{
			RouteNo: result.RouteNo, Status: "signing", Recovered: admission.recovered || result.Recovered,
		})
	}
}

func localTencentContractTerminationHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentContractTerminationsPath ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
		if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
			clearRuntimeBytes(body)
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		defer clearRuntimeBytes(body)
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, admission, _, err := runtime.authenticateRequest(
			request.Context(), processauth.TencentContractTerminationCreateScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeJSON, body, credentials, true,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		var input processauth.TencentContractTerminationRequest
		now := time.Now().UTC()
		if decodeTencentOnboardingJSON(body, &input) != nil ||
			!contractFlowNoPattern.MatchString(input.ContractNo) ||
			!gatewayRoutePattern.MatchString(input.OriginalRouteNo) ||
			!validTencentReleaseText(input.Reason, true) ||
			!validTencentReleaseText(input.RemainInForceItem, false) ||
			!validTencentReleaseText(input.OriginalExpenseSettlement, false) ||
			!validTencentReleaseText(input.OriginalOtherSettlement, false) ||
			!validTencentReleaseText(input.OtherDeals, false) ||
			input.DeadlineUnix <= now.Unix() || input.DeadlineUnix > now.Add(8*24*time.Hour).Unix() {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() ||
			runtime.supplierReconciliation == nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		info := &essbasic.RelieveInfo{
			Reason:                    &input.Reason,
			RemainInForceItem:         optionalStringPointer(input.RemainInForceItem),
			OriginalExpenseSettlement: optionalStringPointer(input.OriginalExpenseSettlement),
			OriginalOtherSettlement:   optionalStringPointer(input.OriginalOtherSettlement),
			OtherDeals:                optionalStringPointer(input.OtherDeals),
		}
		var result tencentess.CreateFlowResult
		err = runtime.supplierReconciliation.WithCompletedReference(
			request.Context(), input.OriginalRouteNo, suppliercallback.ProviderTencentESSBasic,
			tencentess.OperationFlowCreate, func(reference []byte) error {
				var createErr error
				result, createErr = runtime.tencentESS.CreateReleaseFlow(
					request.Context(), admission.requestNo, string(reference), info, input.DeadlineUnix,
				)
				return createErr
			},
		)
		if err != nil {
			if errors.Is(err, suppliercallback.ErrConflict) {
				writeJSON(writer, http.StatusConflict, map[string]string{errorCodeField: "contract_flow_not_terminable"})
				return
			}
			writeTencentContractFlowError(writer, err, result.RouteNo)
			return
		}
		if result.RouteNo == "" {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_result_unknown"})
			return
		}
		writeJSON(writer, http.StatusOK, processauth.TencentContractFlowResponse{
			RouteNo: result.RouteNo, Status: "signing", Recovered: admission.recovered || result.Recovered,
		})
	}
}

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func validTencentReleaseText(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if strings.TrimSpace(value) != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 {
		return false
	}
	for _, character := range value {
		if !(unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsPunct(character) ||
			unicode.IsSpace(character)) {
			return false
		}
	}
	return true
}

func localTencentContractSigningLinkHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, ok := authenticateTencentContractRead(
			writer, request, runtime, processauth.TencentContractSigningLinksPath,
			processauth.TencentContractSigningLinkCreateScope,
		)
		if !ok {
			return
		}
		defer clearRuntimeBytes(body)
		var input processauth.TencentContractSigningLinkRequest
		if decodeTencentOnboardingJSON(body, &input) != nil ||
			!gatewayRoutePattern.MatchString(input.RouteNo) || !validHTTPSContractURL(input.JumpURL) ||
			input.Approver.PartyCode != "customer" || input.Approver.SequenceNo < 1 {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		approver := &essbasic.FlowApproverInfo{RecipientId: &input.Approver.RecipientRef}
		if err := bindTencentCustomerApprover(approver, input.Approver); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() ||
			runtime.supplierReconciliation == nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		var responseURL string
		err := runtime.supplierReconciliation.WithRunningReference(
			request.Context(), input.RouteNo, suppliercallback.ProviderTencentESSBasic,
			tencentContractOperationScope, func(reference []byte) error {
				response, createErr := runtime.tencentESS.CreateSignURL(
					request.Context(), string(reference), []*essbasic.FlowApproverInfo{approver}, input.JumpURL,
				)
				if createErr != nil {
					return createErr
				}
				if response == nil || response.Response == nil ||
					len(response.Response.FlowApproverUrlInfos) != 1 ||
					response.Response.FlowApproverUrlInfos[0] == nil {
					return errors.New("tencent signing URL response is invalid")
				}
				item := response.Response.FlowApproverUrlInfos[0]
				if item.SignUrl != nil {
					responseURL = strings.TrimSpace(*item.SignUrl)
				}
				if responseURL == "" && item.LongUrl != nil {
					responseURL = strings.TrimSpace(*item.LongUrl)
				}
				if !validHTTPSContractURL(responseURL) {
					return errors.New("tencent signing URL response is invalid")
				}
				return nil
			},
		)
		if err != nil {
			writeTencentContractReadError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, processauth.TencentContractSigningLinkResponse{
			URL: responseURL, ExpiresUnix: time.Now().UTC().Add(30 * time.Minute).Unix(),
		})
	}
}

func localTencentContractFlowRecheckHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, ok := authenticateTencentContractRead(
			writer, request, runtime, processauth.TencentContractFlowRechecksPath,
			processauth.TencentContractFlowRecheckScope,
		)
		if !ok {
			return
		}
		defer clearRuntimeBytes(body)
		var input processauth.TencentContractFlowRecheckRequest
		if decodeTencentOnboardingJSON(body, &input) != nil ||
			!gatewayRoutePattern.MatchString(input.RouteNo) {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.supplierReconciliation == nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		result, err := runtime.supplierReconciliation.RunRoute(
			request.Context(), input.RouteNo, suppliercallback.ProviderTencentESSBasic,
			tencentContractOperationScope,
		)
		if err != nil {
			writeTencentContractReadError(writer, err)
			return
		}
		supplierObservedUnix := int64(0)
		if !result.SupplierObservedAt.IsZero() {
			supplierObservedUnix = result.SupplierObservedAt.UTC().Unix()
		}
		writeJSON(writer, http.StatusOK, processauth.TencentContractFlowRecheckResponse{
			RouteNo: result.RouteNo, Status: string(result.ProjectedStatus),
			ProviderStatus:       result.ProviderStatus,
			ReconciliationStatus: result.ReconciliationStatus, RouteVersion: result.RouteVersion,
			SupplierObservedUnix: supplierObservedUnix,
			DecisionCode:         result.DecisionCode,
		})
	}
}

func authenticateTencentContractRead(
	writer http.ResponseWriter,
	request *http.Request,
	runtime *localRuntime,
	path string,
	scope string,
) ([]byte, bool) {
	if request.URL.RequestURI() != path || request.Header.Get("Content-Type") != "application/json" || runtime == nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearRuntimeBytes(body)
		writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
		return nil, false
	}
	credentials, err := credentialsFromRequest(request)
	if err != nil {
		clearRuntimeBytes(body)
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return nil, false
	}
	_, _, err = runtime.authenticateRead(
		request.Context(), scope, request.Method, request.URL.RequestURI(),
		processauth.PayloadModeJSON, body, credentials,
	)
	if err != nil {
		clearRuntimeBytes(body)
		writeProcessAdmissionError(writer, err)
		return nil, false
	}
	return body, true
}

func validHTTPSContractURL(value string) bool {
	if len(value) < 1 || len(value) > 1000 || strings.ContainsAny(value, "\r\n\t\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func writeTencentContractReadError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, suppliercallback.ErrConflict), errors.Is(err, suppliercallback.ErrLeaseLost):
		writeJSON(writer, http.StatusConflict, map[string]string{errorCodeField: "contract_flow_not_signable"})
	case errors.Is(err, suppliercallback.ErrQueryDisabled):
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_query_not_configured"})
	default:
		writeTencentContractFlowError(writer, err, "")
	}
}

func buildTencentContractFlow(
	input processauth.TencentContractFlowRequest,
	now time.Time,
) (*essbasic.FlowInfo, error) {
	if !contractFlowNoPattern.MatchString(input.ContractNo) || !tencentRefPattern.MatchString(input.TemplateRef) ||
		(input.LanguageCode != "zh-CN" && input.LanguageCode != "en-US") || now.IsZero() ||
		input.DeadlineUnix <= now.Unix() || input.DeadlineUnix > now.Add(8*24*time.Hour).Unix() ||
		len(input.FormFields) < 1 || len(input.FormFields) > 256 ||
		len(input.Approvers) < 2 || len(input.Approvers) > 50 {
		return nil, errors.New("tencent contract flow input is invalid")
	}
	formFields, err := buildTencentContractFormFields(input.FormFields)
	if err != nil {
		return nil, err
	}
	approvers, err := buildTencentContractApprovers(input.Approvers)
	if err != nil {
		return nil, err
	}
	flowName := "Contract_" + input.ContractNo
	return &essbasic.FlowInfo{
		FlowName: &flowName, Deadline: &input.DeadlineUnix, TemplateId: &input.TemplateRef,
		FormFields: formFields, FlowApprovers: approvers,
	}, nil
}

func buildTencentContractFormFields(
	sources []processauth.TencentContractFormField,
) ([]*essbasic.FormField, error) {
	fields := make([]*essbasic.FormField, 0, len(sources))
	fieldNames := make(map[string]struct{}, len(sources))
	dynamicTables := 0
	locked := true
	for _, field := range sources {
		if !validTencentContractText(field.ComponentName, 100) ||
			(field.TypeCode != "TEXT" && field.TypeCode != "MULTI_LINE_TEXT" && field.TypeCode != "DYNAMIC_TABLE") ||
			!validTencentContractFieldValue(field.TypeCode, field.Value) {
			return nil, errors.New("tencent contract form field is invalid")
		}
		if _, exists := fieldNames[field.ComponentName]; exists {
			return nil, errors.New("tencent contract form field is duplicated")
		}
		fieldNames[field.ComponentName] = struct{}{}
		if field.TypeCode == "DYNAMIC_TABLE" {
			dynamicTables++
		}
		name := field.ComponentName
		value := field.Value
		fields = append(fields, &essbasic.FormField{
			ComponentName: &name, ComponentValue: &value, LockComponentValue: &locked,
		})
	}
	if dynamicTables != 1 {
		return nil, errors.New("tencent contract service table is invalid")
	}
	return fields, nil
}

func buildTencentContractApprovers(
	sources []processauth.TencentContractApprover,
) ([]*essbasic.FlowApproverInfo, error) {
	approvers := make([]*essbasic.FlowApproverInfo, 0, len(sources))
	customerCount := 0
	merchantCount := 0
	notifyNone := "NONE"
	for index, source := range sources {
		if source.SequenceNo != index+1 || !validTencentContractText(source.RecipientRef, 128) ||
			!validTencentContractText(source.OrganizationName, 200) && source.PartyCode == "merchant" {
			return nil, errors.New("tencent contract approver is invalid")
		}
		approver := &essbasic.FlowApproverInfo{
			RecipientId: &source.RecipientRef, NotifyType: &notifyNone,
		}
		switch source.PartyCode {
		case "customer":
			customerCount++
			if err := bindTencentCustomerApprover(approver, source); err != nil {
				return nil, err
			}
		case "merchant":
			merchantCount++
			if source.SubjectTypeCode != "enterprise" || source.Name != "" || source.Mobile != "" ||
				!validTencentContractText(source.OrganizationName, 200) {
				return nil, errors.New("tencent merchant approver is invalid")
			}
			approverType := "ORGANIZATION"
			notChannelOrganization := false
			approver.ApproverType = &approverType
			approver.OrganizationName = &source.OrganizationName
			approver.NotChannelOrganization = &notChannelOrganization
		default:
			return nil, errors.New("tencent contract approver party is invalid")
		}
		approvers = append(approvers, approver)
	}
	if customerCount < 1 || merchantCount < 1 {
		return nil, errors.New("tencent contract approvers are incomplete")
	}
	return approvers, nil
}

func bindTencentCustomerApprover(
	approver *essbasic.FlowApproverInfo,
	source processauth.TencentContractApprover,
) error {
	if !validTencentContractText(source.Name, 200) || !mobilePattern.MatchString(source.Mobile) {
		return errors.New("tencent contract customer approver is invalid")
	}
	approver.Name = &source.Name
	approver.Mobile = &source.Mobile
	switch source.SubjectTypeCode {
	case "individual":
		if source.OrganizationName != "" {
			return errors.New("tencent personal approver is invalid")
		}
		approverType := "PERSON"
		approver.ApproverType = &approverType
	case "enterprise":
		if !validTencentContractText(source.OrganizationName, 200) {
			return errors.New("tencent enterprise approver is invalid")
		}
		approverType := "ORGANIZATION"
		notChannelOrganization := true
		approver.ApproverType = &approverType
		approver.OrganizationName = &source.OrganizationName
		approver.NotChannelOrganization = &notChannelOrganization
	default:
		return errors.New("tencent customer approver type is invalid")
	}
	return nil
}

func validTencentContractFieldValue(typeCode, value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00") {
		return false
	}
	if typeCode != "DYNAMIC_TABLE" {
		if utf8.RuneCountInString(value) > 1000 {
			return false
		}
		for _, character := range value {
			if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
				return false
			}
		}
		return true
	}
	if len(value) > 256<<10 {
		return false
	}
	var table dynamicTablePayload
	if decodeTencentOnboardingJSON([]byte(value), &table) != nil || len(table.Headers) != 4 ||
		table.RowCount < 1 || table.RowCount > 100 || len(table.Body.Cells) != table.RowCount*4 {
		return false
	}
	for _, header := range table.Headers {
		if !validTencentContractText(header.Content, 100) {
			return false
		}
	}
	seen := make(map[[2]int]struct{}, len(table.Body.Cells))
	for _, cell := range table.Body.Cells {
		position := [2]int{cell.RowStart, cell.ColumnStart}
		if cell.RowStart < 1 || cell.RowStart > table.RowCount || cell.RowEnd != cell.RowStart ||
			cell.ColumnStart < 1 || cell.ColumnStart > 4 || cell.ColumnEnd != cell.ColumnStart ||
			!validTencentContractText(cell.Content, 100) {
			return false
		}
		if _, exists := seen[position]; exists {
			return false
		}
		seen[position] = struct{}{}
	}
	return len(seen) == table.RowCount*4
}

func validTencentContractText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum &&
		!strings.ContainsAny(value, "\r\n\t\x00")
}

func writeTencentContractFlowError(writer http.ResponseWriter, err error, routeNo string) {
	payload := map[string]string{}
	if gatewayRoutePattern.MatchString(routeNo) {
		payload["routeNo"] = routeNo
	}
	var safe *tencentess.Error
	if !errors.As(err, &safe) {
		payload[errorCodeField] = "tencent_result_unknown"
		writeJSON(writer, http.StatusServiceUnavailable, payload)
		return
	}
	status := http.StatusServiceUnavailable
	if safe.Code == tencentess.ErrorInvalidInput {
		status = http.StatusBadRequest
	}
	payload[errorCodeField] = string(safe.Code)
	writeJSON(writer, status, payload)
}
