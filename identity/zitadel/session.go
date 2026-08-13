package zitadel

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	oidcv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/oidc/v2"
	orgv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/org/v2"
	sessionv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/session/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Login exchanges credentials for an OIDC callback: check them as a
// session, finalize the pending auth request with it, and hand the browser
// the URL that completes the flow. Proxying keeps Zitadel off the browser's
// origin and the flow in one place.
func (p *Provider) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	m := req.Msg
	if m.GetAuthRequestId() == "" || m.GetLoginName() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("auth_request_id, login_name, and password are required"))
	}

	loginName := m.GetLoginName()
	session, err := p.api.SessionServiceV2().CreateSession(ctx, &sessionv2.CreateSessionRequest{
		Checks: &sessionv2.Checks{
			User: &sessionv2.CheckUser{Search: &sessionv2.CheckUser_LoginName{LoginName: loginName}},
			Password: &sessionv2.CheckPassword{
				Password: m.GetPassword(),
			},
		},
	})
	if err != nil {
		// Unknown users and wrong passwords both land here; neither should
		// tell the caller which it was.
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	callback, err := p.api.OIDCServiceV2().CreateCallback(ctx, &oidcv2.CreateCallbackRequest{
		AuthRequestId: m.GetAuthRequestId(),
		CallbackKind: &oidcv2.CreateCallbackRequest_Session{
			Session: &oidcv2.Session{
				SessionId:    session.GetSessionId(),
				SessionToken: session.GetSessionToken(),
			},
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.LoginResponse{CallbackUrl: callback.GetCallbackUrl()}), nil
}

// Register creates an organization and its first admin: filament's
// self-service signup. The organization becomes the tenant.
func (p *Provider) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {
	m := req.Msg
	if m.GetOrgName() == "" || m.GetGivenName() == "" || m.GetFamilyName() == "" || m.GetEmail() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("org_name, given_name, family_name, email, and password are required"))
	}

	created, err := p.api.OrganizationServiceV2().AddOrganization(ctx, &orgv2.AddOrganizationRequest{
		Name: m.GetOrgName(),
		Admins: []*orgv2.AddOrganizationRequest_Admin{{
			UserType: &orgv2.AddOrganizationRequest_Admin_Human{
				Human: &userv2.AddHumanUserRequest{
					Profile: &userv2.SetHumanProfile{
						GivenName:  m.GetGivenName(),
						FamilyName: m.GetFamilyName(),
					},
					// Filament sends no mail, so the founder's address
					// starts verified and they can sign in immediately.
					Email: &userv2.SetHumanEmail{
						Email:        m.GetEmail(),
						Verification: &userv2.SetHumanEmail_IsVerified{IsVerified: true},
					},
					PasswordType: &userv2.AddHumanUserRequest_Password{
						Password: &userv2.Password{Password: m.GetPassword()},
					},
				},
			},
		}},
	})
	if err != nil {
		// Duplicate names and password-policy failures both land here with
		// Zitadel's reason attached.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// The founder administers the tenant they just created.
	orgID := created.GetOrganizationId()
	adminRole := identity.RoleKey(authv1.Role_ROLE_ADMIN)
	for _, admin := range created.GetCreatedAdmins() {
		if err := p.grantRole(ctx, orgID, admin.GetUserId(), adminRole); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&authv1.RegisterResponse{TenantId: orgID}), nil
}

// AcceptInvite redeems an invitation and sets the member's password. The
// code is the credential, which is why this RPC is public.
func (p *Provider) AcceptInvite(ctx context.Context, req *connect.Request[authv1.AcceptInviteRequest]) (*connect.Response[authv1.AcceptInviteResponse], error) {
	m := req.Msg
	if m.GetUserId() == "" || m.GetCode() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id, code, and password are required"))
	}
	if _, err := p.api.UserServiceV2().VerifyInviteCode(ctx, &userv2.VerifyInviteCodeRequest{
		UserId:           m.GetUserId(),
		VerificationCode: m.GetCode(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired invite"))
	}
	if _, err := p.api.UserServiceV2().UpdateUser(ctx, &userv2.UpdateUserRequest{
		UserId: m.GetUserId(),
		UserType: &userv2.UpdateUserRequest_Human_{
			Human: &userv2.UpdateUserRequest_Human{
				Password: &userv2.SetPassword{
					PasswordType: &userv2.SetPassword_Password{
						Password: &userv2.Password{Password: m.GetPassword(), ChangeRequired: false},
					},
				},
			},
		},
	}); err != nil {
		// Password-policy failures surface with Zitadel's reason.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&authv1.AcceptInviteResponse{}), nil
}
