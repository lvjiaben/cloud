// This file verifies publication preview dependency and authorization boundaries.

package catalog

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
)

type publicationTestAutomationReadiness struct {
	ready bool
	err   error
	calls int
}

func (r *publicationTestAutomationReadiness) CheckPublicationReadiness(context.Context, string) (bool, error) {
	r.calls++
	return r.ready, r.err
}

func TestNewPublicationPreviewRequiresFailClosedDependencies(t *testing.T) {
	authorizer := &publicationTestAuthorizer{}
	readiness := &publicationTestAutomationReadiness{}
	if _, err := NewPublicationPreview(nil, readiness); err == nil {
		t.Fatal("nil publication preview authorizer accepted")
	}
	if _, err := NewPublicationPreview(authorizer, nil); err == nil {
		t.Fatal("nil publication preview automation readiness accepted")
	}
}

func TestPublicationPreviewAuthorizesBeforeInputAndPersistence(t *testing.T) {
	authorizationError := errors.New("publication preview denied")
	authorizer := &publicationTestAuthorizer{err: authorizationError}
	readiness := &publicationTestAutomationReadiness{ready: true}
	service, err := NewPublicationPreview(authorizer, readiness)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.PreviewVersion(context.Background(), "invalid target"); !errors.Is(err, authorizationError) {
		t.Fatalf("publication preview returned %v, want authorization error", err)
	}
	if authorizer.calls != 1 {
		t.Fatalf("publication preview authorization calls=%d want=1", authorizer.calls)
	}
	if readiness.calls != 0 {
		t.Fatalf("automation readiness ran before publication authorization: calls=%d", readiness.calls)
	}
}

func TestPublicationPreviewAutomationErrorClassification(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		readiness *publicationTestAutomationReadiness
		want      bool
		wantErr   *bizerr.Code
	}{
		{name: "accepting or unbound", readiness: &publicationTestAutomationReadiness{ready: true}, want: true},
		{name: "stopped", readiness: &publicationTestAutomationReadiness{}, want: false},
		{name: "evidence conflict", readiness: &publicationTestAutomationReadiness{err: bizerr.NewCode(CodeManagementConflict)}, want: false},
		{name: "host unavailable", readiness: &publicationTestAutomationReadiness{err: errors.New("host unavailable")}, wantErr: CodePublicationUnavailable},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := &publicationPreviewServiceImpl{automation: testCase.readiness}
			ready, err := service.publicationAutomationReady(context.Background(), "ver_0123456789abcdef0123456789abcdef")
			if testCase.wantErr != nil {
				if !bizerr.Is(err, testCase.wantErr) {
					t.Fatalf("automation error=%v want=%s", err, testCase.wantErr.RuntimeCode())
				}
				if bizerr.Is(err, CodeManagementUnavailable) || bizerr.Is(err, CodeManagementConflict) {
					t.Fatalf("automation management detail leaked: %v", err)
				}
				return
			}
			if err != nil || ready != testCase.want {
				t.Fatalf("automation readiness=%v err=%v want=%v", ready, err, testCase.want)
			}
		})
	}
}
