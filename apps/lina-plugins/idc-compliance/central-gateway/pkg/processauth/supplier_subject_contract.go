// This file freezes the signed supplier-subject routes shared by both local processes.

package processauth

// Supplier-subject routes and scopes are shared by the colocated plugin signer
// and local gateway verifier so either side cannot silently drift.
const (
	// SupplierSubjectBindingsPath creates one installation-local subject binding.
	SupplierSubjectBindingsPath = "/internal/v1/supplier-subject-bindings"
	// SupplierSubjectRevocationsPath creates one installation-local revocation record.
	SupplierSubjectRevocationsPath = "/internal/v1/supplier-subject-revocations"
	// SupplierSubjectReopeningsPath creates one installation-local reopening record.
	SupplierSubjectReopeningsPath = "/internal/v1/supplier-subject-reopenings"
	// SupplierSubjectBindScope authenticates only the binding route.
	SupplierSubjectBindScope = "supplier-subject.bind"
	// SupplierSubjectRevokeScope authenticates only the revocation route.
	SupplierSubjectRevokeScope = "supplier-subject.revoke"
	// SupplierSubjectReopenScope authenticates only the reopening route.
	SupplierSubjectReopenScope = "supplier-subject.reopen"
)
