package zitadel

import (
	"context"
	"fmt"
	"slices"

	appv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/application/v2"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/auth"
	authorizationv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/authorization/v2"
	featurev2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/feature/v2"
	permissionv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/internal_permission/v2"
	projectv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/project/v2"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/galaxy-io/filament/identity"
)

const (
	projectName = "filament"
	appName     = "filament-ui"
	// loginClientRole lets the machine user finalize auth requests on behalf
	// of filament's own login page.
	loginClientRole = "IAM_LOGIN_CLIENT"
)

// projectRef is filament's project and the organization owning it. Other
// organizations need the project granted to them before their users can
// hold filament roles.
type projectRef struct {
	id         string
	ownerOrgID string
}

// bootstrap converges the instance on the configuration filament needs and
// caches the project and OIDC client id on the provider. Every step is
// idempotent, so each boot lands on the same state.
func (p *Provider) bootstrap(ctx context.Context, uiOrigin string) error {
	// A fresh instance forces every app onto the built-in login UI; routing
	// to filament's own page only applies with that force off.
	if _, err := p.api.FeatureServiceV2().SetInstanceFeatures(ctx, &featurev2.SetInstanceFeaturesRequest{
		LoginV2: &featurev2.LoginV2{Required: false},
	}); err != nil {
		return fmt.Errorf("disable forced login v2: %w", err)
	}
	// The machine user's own organization owns filament's project.
	me, err := p.api.AuthService().GetMyUser(ctx, &auth.GetMyUserRequest{})
	if err != nil {
		return fmt.Errorf("resolve machine user: %w", err)
	}
	homeOrgID := me.GetUser().GetDetails().GetResourceOwner()
	if err := p.ensureLoginClientRole(ctx, me.GetUser().GetId()); err != nil {
		return fmt.Errorf("grant %s: %w", loginClientRole, err)
	}
	if err := p.ensureProject(ctx, homeOrgID); err != nil {
		return err
	}
	if err := p.ensureProjectRoles(ctx); err != nil {
		return err
	}
	return p.ensureApp(ctx, uiOrigin)
}

// ensureLoginClientRole grants the machine user IAM_LOGIN_CLIENT alongside
// the instance roles it already holds; a first-instance user only gets
// IAM_OWNER.
func (p *Provider) ensureLoginClientRole(ctx context.Context, userID string) error {
	admins, err := p.api.InternalPermissionServiceV2().ListAdministrators(ctx, &permissionv2.ListAdministratorsRequest{})
	if err != nil {
		return err
	}
	for _, administrator := range admins.GetAdministrators() {
		// Instance-level grants are the only ones that carry this role.
		if administrator.GetUser().GetId() != userID || !administrator.GetInstance() {
			continue
		}
		if slices.Contains(administrator.GetRoles(), loginClientRole) {
			return nil
		}
		_, err := p.api.InternalPermissionServiceV2().UpdateAdministrator(ctx, &permissionv2.UpdateAdministratorRequest{
			UserId:   userID,
			Resource: instanceScope(),
			Roles:    append(administrator.GetRoles(), loginClientRole),
		})
		return err
	}
	_, err = p.api.InternalPermissionServiceV2().CreateAdministrator(ctx, &permissionv2.CreateAdministratorRequest{
		UserId:   userID,
		Resource: instanceScope(),
		Roles:    []string{loginClientRole},
	})
	return err
}

// instanceScope targets administrator grants at the whole instance, which is
// where IAM roles live.
func instanceScope() *permissionv2.ResourceType {
	return &permissionv2.ResourceType{
		Resource: &permissionv2.ResourceType_Instance{Instance: true},
	}
}

// ensureProject finds or creates filament's project. Role assertion puts
// granted roles in every token its applications issue.
func (p *Provider) ensureProject(ctx context.Context, homeOrgID string) error {
	found, err := p.api.ProjectServiceV2().ListProjects(ctx, &projectv2.ListProjectsRequest{
		Filters: []*projectv2.ProjectSearchFilter{{
			Filter: &projectv2.ProjectSearchFilter_ProjectNameFilter{
				ProjectNameFilter: &projectv2.ProjectNameFilter{ProjectName: projectName},
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	if result := found.GetProjects(); len(result) > 0 {
		p.project = projectRef{id: result[0].GetProjectId(), ownerOrgID: result[0].GetOrganizationId()}
		return nil
	}
	created, err := p.api.ProjectServiceV2().CreateProject(ctx, &projectv2.CreateProjectRequest{
		OrganizationId:       homeOrgID,
		Name:                 projectName,
		ProjectRoleAssertion: true,
	})
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	p.project = projectRef{id: created.GetProjectId(), ownerOrgID: homeOrgID}
	return nil
}

// ensureProjectRoles adds any filament role the project is missing.
func (p *Provider) ensureProjectRoles(ctx context.Context) error {
	existing, err := p.api.ProjectServiceV2().ListProjectRoles(ctx, &projectv2.ListProjectRolesRequest{
		ProjectId: p.project.id,
	})
	if err != nil {
		return fmt.Errorf("list project roles: %w", err)
	}
	have := make(map[string]bool, len(existing.GetProjectRoles()))
	for _, role := range existing.GetProjectRoles() {
		have[role.GetKey()] = true
	}
	for _, key := range identity.RoleKeys() {
		if have[key] {
			continue
		}
		if _, err := p.api.ProjectServiceV2().AddProjectRole(ctx, &projectv2.AddProjectRoleRequest{
			ProjectId:   p.project.id,
			RoleKey:     key,
			DisplayName: key,
		}); err != nil {
			return fmt.Errorf("add project role %s: %w", key, err)
		}
	}
	return nil
}

// ensureApp finds or creates the SPA registration and keeps its login base
// URI tracking the configured UI origin.
func (p *Provider) ensureApp(ctx context.Context, uiOrigin string) error {
	found, err := p.api.ApplicationServiceV2().ListApplications(ctx, &appv2.ListApplicationsRequest{
		Filters: []*appv2.ApplicationSearchFilter{
			{Filter: &appv2.ApplicationSearchFilter_ProjectIdFilter{
				ProjectIdFilter: &appv2.ProjectIDFilter{ProjectId: p.project.id},
			}},
			{Filter: &appv2.ApplicationSearchFilter_NameFilter{
				NameFilter: &appv2.ApplicationNameFilter{Name: appName},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("list applications: %w", err)
	}
	if result := found.GetApplications(); len(result) > 0 {
		existing := result[0]
		p.clientID = existing.GetOidcConfiguration().GetClientId()
		if existing.GetOidcConfiguration().GetLoginVersion().GetLoginV2().GetBaseUri() == uiOrigin {
			return nil
		}
		_, err := p.api.ApplicationServiceV2().UpdateApplication(ctx, &appv2.UpdateApplicationRequest{
			ProjectId:     p.project.id,
			ApplicationId: existing.GetApplicationId(),
			ApplicationType: &appv2.UpdateApplicationRequest_OidcConfiguration{
				OidcConfiguration: updateOIDCConfig(uiOrigin),
			},
		})
		return err
	}
	created, err := p.api.ApplicationServiceV2().CreateApplication(ctx, &appv2.CreateApplicationRequest{
		ProjectId: p.project.id,
		Name:      appName,
		ApplicationType: &appv2.CreateApplicationRequest_OidcConfiguration{
			OidcConfiguration: createOIDCConfig(uiOrigin),
		},
	})
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}
	p.clientID = created.GetOidcConfiguration().GetClientId()
	return nil
}

// loginVersion routes the authorize endpoint at filament's own login page.
// Zitadel appends /login to the base URI when it redirects.
func loginVersion(uiOrigin string) *appv2.LoginVersion {
	return &appv2.LoginVersion{
		Version: &appv2.LoginVersion_LoginV2{LoginV2: &appv2.LoginV2{BaseUri: &uiOrigin}},
	}
}

// createOIDCConfig registers the SPA: PKCE with no client secret, code flow
// only, and JWT access tokens so the API verifies against JWKS instead of
// introspecting. Roles ride in the token so authorization needs no lookup.
func createOIDCConfig(uiOrigin string) *appv2.CreateOIDCApplicationRequest {
	return &appv2.CreateOIDCApplicationRequest{
		RedirectUris:             []string{uiOrigin + "/auth/callback"},
		PostLogoutRedirectUris:   []string{uiOrigin},
		ResponseTypes:            []appv2.OIDCResponseType{appv2.OIDCResponseType_OIDC_RESPONSE_TYPE_CODE},
		GrantTypes:               []appv2.OIDCGrantType{appv2.OIDCGrantType_OIDC_GRANT_TYPE_AUTHORIZATION_CODE, appv2.OIDCGrantType_OIDC_GRANT_TYPE_REFRESH_TOKEN},
		ApplicationType:          appv2.OIDCApplicationType_OIDC_APP_TYPE_USER_AGENT,
		AuthMethodType:           appv2.OIDCAuthMethodType_OIDC_AUTH_METHOD_TYPE_NONE,
		AccessTokenType:          appv2.OIDCTokenType_OIDC_TOKEN_TYPE_JWT,
		AccessTokenRoleAssertion: true,
		DevelopmentMode:          true,
		LoginVersion:             loginVersion(uiOrigin),
	}
}

func updateOIDCConfig(uiOrigin string) *appv2.UpdateOIDCApplicationConfigurationRequest {
	appType := appv2.OIDCApplicationType_OIDC_APP_TYPE_USER_AGENT
	authMethod := appv2.OIDCAuthMethodType_OIDC_AUTH_METHOD_TYPE_NONE
	tokenType := appv2.OIDCTokenType_OIDC_TOKEN_TYPE_JWT
	enabled := true
	return &appv2.UpdateOIDCApplicationConfigurationRequest{
		RedirectUris:             []string{uiOrigin + "/auth/callback"},
		PostLogoutRedirectUris:   []string{uiOrigin},
		ResponseTypes:            []appv2.OIDCResponseType{appv2.OIDCResponseType_OIDC_RESPONSE_TYPE_CODE},
		GrantTypes:               []appv2.OIDCGrantType{appv2.OIDCGrantType_OIDC_GRANT_TYPE_AUTHORIZATION_CODE, appv2.OIDCGrantType_OIDC_GRANT_TYPE_REFRESH_TOKEN},
		ApplicationType:          &appType,
		AuthMethodType:           &authMethod,
		AccessTokenType:          &tokenType,
		AccessTokenRoleAssertion: &enabled,
		DevelopmentMode:          &enabled,
		LoginVersion:             loginVersion(uiOrigin),
	}
}

// ensureProjectGrant shares filament's project with an organization once, so
// its users can hold filament roles. The owning organization needs none.
func (p *Provider) ensureProjectGrant(ctx context.Context, orgID string) error {
	if orgID == p.project.ownerOrgID {
		return nil
	}
	if _, granted := p.projectGrants.Load(orgID); granted {
		return nil
	}
	_, err := p.api.ProjectServiceV2().CreateProjectGrant(ctx, &projectv2.CreateProjectGrantRequest{
		ProjectId:             p.project.id,
		GrantedOrganizationId: orgID,
		RoleKeys:              identity.RoleKeys(),
	})
	// An existing grant is the desired state, not a failure; a later
	// authorization call surfaces anything genuinely broken.
	p.projectGrants.Store(orgID, struct{}{})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

// grantRole gives a user one filament role in their organization.
func (p *Provider) grantRole(ctx context.Context, orgID, userID, roleKey string) error {
	if err := p.ensureProjectGrant(ctx, orgID); err != nil {
		return err
	}
	_, err := p.api.AuthorizationServiceV2().CreateAuthorization(ctx, &authorizationv2.CreateAuthorizationRequest{
		UserId:         userID,
		ProjectId:      p.project.id,
		OrganizationId: orgID,
		RoleKeys:       []string{roleKey},
	})
	return err
}

// isAlreadyExists reports whether an error means the resource filament was
// about to create is already there, which is the state it wanted anyway.
func isAlreadyExists(err error) bool {
	return status.Code(err) == codes.AlreadyExists
}
