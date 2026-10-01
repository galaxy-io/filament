package keycloak

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Invitations live on the invited user as attributes: the code's hash and
// its expiry. Keycloak's own invitation flow mails the code, and filament
// sends no mail.
const (
	inviteHashKey    = "filament.invite"
	inviteExpiresKey = "filament.invite_expires"
	inviteLifetime   = 7 * 24 * time.Hour
	// serviceAccountPrefix is how Keycloak names a client's service account
	// user, which is how one is told apart from a human in a member list.
	serviceAccountPrefix = "service-account-"
)

func isServiceAccount(user userRep) bool {
	return strings.HasPrefix(user.Username, serviceAccountPrefix)
}

// rolesByUser maps every holder of a filament role to the role they hold.
func (p *Provider) rolesByUser(ctx context.Context) (map[string]authv1.Role, error) {
	roles := map[string]authv1.Role{}
	for _, key := range identity.RoleKeys() {
		holders, err := p.admin.roleUsers(ctx, p.client, key)
		if err != nil {
			return nil, err
		}
		for _, user := range holders {
			roles[user.ID] = identity.RoleFromKey(key)
		}
	}
	return roles, nil
}

// ListMembers returns the tenant's users joined with the filament role each
// holds. The organization's member list names them; a per-user read fills
// in the profile it leaves out.
func (p *Provider) ListMembers(ctx context.Context, _ *connect.Request[authv1.ListMembersRequest]) (*connect.Response[authv1.ListMembersResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	org, err := p.tenant(ctx, caller)
	if err != nil {
		return nil, err
	}
	members, err := p.admin.members(ctx, org.ID)
	if err != nil {
		return nil, rpcError(err)
	}
	roles, err := p.rolesByUser(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*authv1.Member, 0, len(members))
	for _, member := range members {
		if isServiceAccount(member) {
			continue
		}
		user, err := p.admin.getUser(ctx, member.ID)
		if err != nil {
			return nil, rpcError(err)
		}
		out = append(out, &authv1.Member{
			UserId: user.ID,
			Name:   displayName(user),
			Email:  user.Email,
			Role:   roles[user.ID],
		})
	}
	return connect.NewResponse(&authv1.ListMembersResponse{
		Members:   out,
		CanManage: caller.IsAdmin(),
	}), nil
}

// requireMember confirms a user belongs to the caller's tenant, so one
// tenant can never reach another's members.
func (p *Provider) requireMember(ctx context.Context, org organizationRep, userID string) error {
	orgs, err := p.admin.memberOrganizations(ctx, userID)
	if err != nil {
		if isStatus(err, 404) {
			return connect.NewError(connect.CodeNotFound, errors.New("member not found"))
		}
		return rpcError(err)
	}
	for _, candidate := range orgs {
		if candidate.ID == org.ID {
			return nil
		}
	}
	return connect.NewError(connect.CodeNotFound, errors.New("member not found"))
}

// InviteMember creates a teammate in the caller's tenant and returns the
// code that redeems their invitation. Nothing is emailed; the admin shares
// the link.
func (p *Provider) InviteMember(ctx context.Context, req *connect.Request[authv1.InviteMemberRequest]) (*connect.Response[authv1.InviteMemberResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.GetEmail() == "" || m.GetGivenName() == "" || m.GetFamilyName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email, given_name, and family_name are required"))
	}
	roleKey := identity.RoleKey(m.GetRole())
	if roleKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	org, err := p.tenant(ctx, caller)
	if err != nil {
		return nil, err
	}
	code, attributes, err := newInvite()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	userID, err := p.admin.createUser(ctx, userRep{
		Username:   m.GetEmail(),
		Email:      m.GetEmail(),
		FirstName:  m.GetGivenName(),
		LastName:   m.GetFamilyName(),
		Enabled:    true,
		Attributes: attributes,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	if err := p.admin.addMember(ctx, org.ID, userID); err != nil {
		return nil, rpcError(p.discardUser(ctx, userID, err))
	}
	if err := p.grantRole(ctx, userID, roleKey); err != nil {
		return nil, rpcError(p.discardUser(ctx, userID, err))
	}
	return connect.NewResponse(&authv1.InviteMemberResponse{
		UserId: userID,
		Code:   code,
	}), nil
}

// newInvite mints an invitation code and the attributes that hold its hash
// and expiry on the invited user.
func newInvite() (code string, attributes map[string][]string, err error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, err
	}
	code = hex.EncodeToString(raw[:])
	return code, map[string][]string{
		inviteHashKey:    {inviteHash(code)},
		inviteExpiresKey: {strconv.FormatInt(time.Now().Add(inviteLifetime).Unix(), 10)},
	}, nil
}

func inviteHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// discardUser deletes a user whose creation did not complete, so the email
// is free for a retry rather than locked behind a half-created account. It
// runs detached from the request so a dropped client still triggers cleanup.
// cause is returned as-is on success; a failed delete is joined onto it, since
// the admin then has to remove the user by hand.
func (p *Provider) discardUser(ctx context.Context, userID string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := p.admin.deleteUser(ctx, userID); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup of user %s failed, remove it manually: %w", userID, err))
	}
	return cause
}

// AcceptInvite redeems an invitation and sets the member's password. The
// code is the credential, which is why this RPC is public.
func (p *Provider) AcceptInvite(ctx context.Context, req *connect.Request[authv1.AcceptInviteRequest]) (*connect.Response[authv1.AcceptInviteResponse], error) {
	m := req.Msg
	if m.GetUserId() == "" || m.GetCode() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id, code, and password are required"))
	}
	invalid := connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired invite"))
	user, err := p.admin.getUserRaw(ctx, m.GetUserId())
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		return nil, invalid
	}
	attributes, _ := user["attributes"].(map[string]any)
	hash := attribute(attributes, inviteHashKey)
	expires, _ := strconv.ParseInt(attribute(attributes, inviteExpiresKey), 10, 64)
	if hash == "" || subtle.ConstantTimeCompare([]byte(hash), []byte(inviteHash(m.GetCode()))) != 1 || time.Now().Unix() > expires {
		return nil, invalid
	}
	if err := p.admin.resetPassword(ctx, m.GetUserId(), m.GetPassword()); err != nil {
		// Password-policy failures surface with Keycloak's reason.
		return nil, rpcError(err)
	}
	// The code is spent, and the address it went to is now proven.
	delete(attributes, inviteHashKey)
	delete(attributes, inviteExpiresKey)
	user["attributes"] = attributes
	user["emailVerified"] = true
	if err := p.admin.putUser(ctx, m.GetUserId(), user); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.AcceptInviteResponse{}), nil
}

// attribute reads the first value of a user attribute as Keycloak returns
// them: a list per key.
func attribute(attributes map[string]any, key string) string {
	values, _ := attributes[key].([]any)
	if len(values) == 0 {
		return ""
	}
	value, _ := values[0].(string)
	return value
}

// SetMemberRole reassigns a member's role.
func (p *Provider) SetMemberRole(ctx context.Context, req *connect.Request[authv1.SetMemberRoleRequest]) (*connect.Response[authv1.SetMemberRoleResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	roleKey := identity.RoleKey(m.GetRole())
	if roleKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	org, err := p.tenant(ctx, caller)
	if err != nil {
		return nil, err
	}
	if err := p.requireMember(ctx, org, m.GetUserId()); err != nil {
		return nil, err
	}
	if err := p.grantRole(ctx, m.GetUserId(), roleKey); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.SetMemberRoleResponse{}), nil
}

// RemoveMember deletes a member of the caller's tenant. Callers cannot
// remove themselves, and members of other tenants are unreachable.
func (p *Provider) RemoveMember(ctx context.Context, req *connect.Request[authv1.RemoveMemberRequest]) (*connect.Response[authv1.RemoveMemberResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if userID == caller.UserID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("you cannot remove yourself"))
	}
	org, err := p.tenant(ctx, caller)
	if err != nil {
		return nil, err
	}
	if err := p.requireMember(ctx, org, userID); err != nil {
		return nil, err
	}
	if err := p.admin.deleteUser(ctx, userID); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.RemoveMemberResponse{}), nil
}
