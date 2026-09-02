package zitadel

import (
	"context"
	"fmt"

	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/auth"
	authorizationv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/authorization/v2"
	projectv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/project/v2"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/galaxy-io/filament/identity"
)

const projectName = "filament"

// projectRef is filament's project and the organization owning it. Other
// organizations need the project granted to them before their users can
// hold filament roles.
type projectRef struct {
	id         string
	ownerOrgID string
}

// bootstrap converges the instance on the configuration filament needs and
// caches the project on the provider. Every step is idempotent, so each boot
// lands on the same state.
func (p *Provider) bootstrap(ctx context.Context) error {
	// The machine user's own organization owns filament's project.
	me, err := p.api.AuthService().GetMyUser(ctx, &auth.GetMyUserRequest{})
	if err != nil {
		return fmt.Errorf("resolve machine user: %w", err)
	}
	if err := p.ensureProject(ctx, me.GetUser().GetDetails().GetResourceOwner()); err != nil {
		return err
	}
	return p.ensureProjectRoles(ctx)
}

// ensureProject finds or creates filament's project. Role assertion puts
// granted roles in every token issued for it, which is how service accounts
// carry theirs.
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

// isUnavailable reports whether an error means Zitadel could not be reached
// at all, which must never be mistaken for a rejected credential.
func isUnavailable(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}
