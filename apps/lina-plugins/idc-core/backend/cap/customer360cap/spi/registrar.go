// Package spi owns startup-only declarations for customer 360 contributors.
package spi

import (
	"sort"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type registrar struct {
	mu           sync.Mutex
	frozen       bool
	contributors []customer360cap.Contributor
}

var startupRegistrar = &registrar{}

func Declare(contributor customer360cap.Contributor) error {
	return startupRegistrar.declare(contributor)
}

func Freeze() ([]customer360cap.Contributor, error) {
	return startupRegistrar.freeze()
}

func (r *registrar) declare(contributor customer360cap.Contributor) error {
	if r == nil || contributor == nil {
		return gerror.New("customer 360 contributor is required")
	}
	codes := contributor.Codes()
	if len(codes) == 0 {
		return gerror.New("customer 360 contributor must own at least one code")
	}
	owned := make(map[customer360cap.Code]struct{}, len(codes))
	for _, code := range codes {
		if !validCode(code) {
			return gerror.New("customer 360 contributor declares an unknown code")
		}
		if _, duplicate := owned[code]; duplicate {
			return gerror.New("customer 360 contributor declares a duplicate code")
		}
		owned[code] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("customer 360 contributor registrar is frozen")
	}
	for _, existing := range r.contributors {
		for _, code := range existing.Codes() {
			if _, duplicate := owned[code]; duplicate {
				return gerror.New("customer 360 summary code already has an owner")
			}
		}
	}
	r.contributors = append(r.contributors, contributor)
	return nil
}

func (r *registrar) freeze() ([]customer360cap.Contributor, error) {
	if r == nil {
		return nil, gerror.New("customer 360 contributor registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	covered := make(map[customer360cap.Code]struct{}, 9)
	for _, contributor := range r.contributors {
		for _, code := range contributor.Codes() {
			covered[code] = struct{}{}
		}
	}
	if len(covered) != 9 {
		return nil, gerror.New("customer 360 contributors do not cover every required summary")
	}
	result := append([]customer360cap.Contributor(nil), r.contributors...)
	sort.Slice(result, func(i, j int) bool { return result[i].Codes()[0] < result[j].Codes()[0] })
	return result, nil
}

func validCode(code customer360cap.Code) bool {
	switch code {
	case customer360cap.CodeOrders, customer360cap.CodeBills,
		customer360cap.CodeTransactions, customer360cap.CodeServers,
		customer360cap.CodeContracts, customer360cap.CodeTaxInvoices,
		customer360cap.CodeTickets, customer360cap.CodeNotifications,
		customer360cap.CodeAudits:
		return true
	default:
		return false
	}
}
