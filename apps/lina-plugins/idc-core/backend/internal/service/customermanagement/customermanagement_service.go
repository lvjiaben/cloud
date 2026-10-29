// This file composes authorization, validation, and persistence for customer reads.

package customermanagement

import (
	"context"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func newService(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	persistence persistence,
	config Config,
) (Service, error) {
	if nilInterface(bizCtxSvc) || nilInterface(principalSvc) || nilInterface(authorization) ||
		nilInterface(persistence) {
		return nil, gerror.New("customer management reader requires host context, principal, authorization, and persistence services")
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	return &serviceImpl{
		bizCtxSvc: bizCtxSvc, principal: principalSvc, authorization: authorization,
		persistence: persistence, config: config,
	}, nil
}

// Search authorizes the current staff request before executing a bounded query.
func (s *serviceImpl) Search(ctx context.Context, input SearchInput) (*Page, error) {
	if err := s.authorize(ctx, permissionCustomerList); err != nil {
		return nil, err
	}
	normalized, err := normalizeSearch(input, s.config.MaximumPageSize)
	if err != nil {
		return nil, err
	}
	page, err := s.persistence.search(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	if err = validatePage(page, s.config.MaximumPageSize, s.config.MaximumResponseBytes); err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return page, nil
}

// GetOverview authorizes before target validation and persistence access.
func (s *serviceImpl) GetOverview(ctx context.Context, customerNo string) (*Overview, error) {
	if err := s.authorize(ctx, permissionCustomerQuery); err != nil {
		return nil, err
	}
	if !validPersistedCustomerNo(customerNo) {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	overview, err := s.persistence.getOverview(ctx, customerNo)
	if err != nil {
		if bizerr.Is(err, customer.CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	if overview != nil && overview.Balances == nil {
		overview.Balances = make([]BalanceSummary, 0)
	}
	if s.business != nil {
		summary, summaryErr := s.business.Get(ctx, customerNo)
		if summaryErr != nil {
			return nil, bizerr.WrapCode(summaryErr, customer.CodeUnavailable)
		}
		overview.Business = BusinessSummary{
			Orders: summary.Orders, Bills: summary.Bills, Transactions: summary.Transactions,
			Servers: summary.Servers, TaxInvoices: summary.TaxInvoices,
			Tickets: summary.Tickets, Notifications: summary.Notifications, Audits: summary.Audits,
		}
		overview.Balances = make([]BalanceSummary, 0, len(summary.Balances))
		for _, balance := range summary.Balances {
			overview.Balances = append(overview.Balances, BalanceSummary{
				AccountTypeCode: balance.AccountTypeCode, CurrencyCode: balance.CurrencyCode,
				StatusCode: balance.StatusCode, BalanceMinor: balance.BalanceMinor,
				HeldBalanceMinor:      balance.HeldBalanceMinor,
				AvailableBalanceMinor: balance.AvailableBalanceMinor, UpdatedAt: balance.UpdatedAt,
			})
		}
	}
	if err = validateOverview(overview, s.config.MaximumResponseBytes); err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return overview, nil
}

// SearchSegments authorizes before validating filters and executing one indexed page query.
func (s *serviceImpl) SearchSegments(ctx context.Context, input SegmentSearchInput) (*SegmentPage, error) {
	if err := s.authorize(ctx, permissionCustomerList); err != nil {
		return nil, err
	}
	normalized, err := normalizeSegmentSearch(input, s.config.MaximumPageSize)
	if err != nil {
		return nil, err
	}
	page, err := s.persistence.searchSegments(ctx, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	if err = validateSegmentPage(page, s.config.MaximumPageSize, s.config.MaximumResponseBytes); err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return page, nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
