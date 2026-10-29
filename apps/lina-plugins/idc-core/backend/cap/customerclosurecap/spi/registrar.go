// Package spi owns startup-only declarations for closure precheck contributors.
package spi

import (
	"sort"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
)

type registrar struct {
	mu           sync.Mutex
	frozen       bool
	contributors []customerclosurecap.Contributor
}

var startupRegistrar = &registrar{}

// Declare registers one domain owner before the route graph is frozen.
func Declare(contributor customerclosurecap.Contributor) error {
	return startupRegistrar.declare(contributor)
}

// Freeze closes registration and returns a detached, deterministic snapshot.
func Freeze() ([]customerclosurecap.Contributor, error) {
	return startupRegistrar.freeze()
}

func (r *registrar) declare(contributor customerclosurecap.Contributor) error {
	if r == nil || contributor == nil {
		return gerror.New("customer closure contributor is required")
	}
	codes := contributor.Codes()
	if len(codes) == 0 {
		return gerror.New("customer closure contributor must own at least one code")
	}
	seen := make(map[customerclosurecap.Code]struct{}, len(codes))
	for _, code := range codes {
		if !validCode(code) {
			return gerror.New("customer closure contributor declares an unknown code")
		}
		if _, exists := seen[code]; exists {
			return gerror.New("customer closure contributor declares a duplicate code")
		}
		seen[code] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.New("customer closure contributor registrar is frozen")
	}
	for _, existing := range r.contributors {
		for _, existingCode := range existing.Codes() {
			if _, duplicate := seen[existingCode]; duplicate {
				return gerror.New("customer closure finding code already has an owner")
			}
		}
	}
	r.contributors = append(r.contributors, contributor)
	return nil
}

func (r *registrar) freeze() ([]customerclosurecap.Contributor, error) {
	if r == nil {
		return nil, gerror.New("customer closure contributor registrar is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	covered := make(map[customerclosurecap.Code]struct{}, 5)
	for _, contributor := range r.contributors {
		for _, code := range contributor.Codes() {
			covered[code] = struct{}{}
		}
	}
	if len(covered) != 5 {
		return nil, gerror.New("customer closure contributors do not cover every required finding")
	}
	result := append([]customerclosurecap.Contributor(nil), r.contributors...)
	sort.Slice(result, func(i, j int) bool { return result[i].Codes()[0] < result[j].Codes()[0] })
	return result, nil
}

func validCode(code customerclosurecap.Code) bool {
	switch code {
	case customerclosurecap.CodeUnsettledFunds,
		customerclosurecap.CodeActiveServices,
		customerclosurecap.CodeUnsettledInvoices,
		customerclosurecap.CodeInProgressOperations,
		customerclosurecap.CodeLegalRetention:
		return true
	default:
		return false
	}
}
