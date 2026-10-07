// This file adapts the established catalog publication authority without
// deciding any of the still-unconfirmed automation-management permissions.

package productautomation

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type publicationOnlyAuthorizer struct {
	publication catalog.PublicationAuthorizer
}

var _ Authorizer = (*publicationOnlyAuthorizer)(nil)

// NewPublicationOnlyAuthorizer creates a production adapter that authorizes
// only the already-established product publication operation. Every other
// automation-management action fails closed until its permission is confirmed.
func NewPublicationOnlyAuthorizer(publication catalog.PublicationAuthorizer) (Authorizer, error) {
	if nilDependency(publication) {
		return nil, gerror.New("product automation publication authorizer requires catalog publication authority")
	}
	return &publicationOnlyAuthorizer{publication: publication}, nil
}

func (a *publicationOnlyAuthorizer) AuthorizePublicationAdmission(ctx context.Context, versionNo string) (Authorization, error) {
	if a == nil || nilDependency(a.publication) {
		return Authorization{}, bizerr.NewCode(catalog.CodePublicationNotAccessible)
	}
	actorUserID, err := a.publication.AuthorizeProductPublication(ctx, versionNo)
	if err != nil {
		return Authorization{}, err
	}
	return Authorization{ActorUserID: actorUserID, RequiredPermission: catalog.PermissionCatalogProductPublish}, nil
}

func (*publicationOnlyAuthorizer) AuthorizeProviderCandidates(context.Context) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceList(context.Context) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceCreate(context.Context) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceRename(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceDelete(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceStop(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceResume(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeInterfaceConfiguration(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeBindingRead(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeBindingWrite(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func (*publicationOnlyAuthorizer) AuthorizeVersionConfiguration(context.Context, string) (Authorization, error) {
	return publicationManagementDenied()
}

func publicationManagementDenied() (Authorization, error) {
	return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
}
