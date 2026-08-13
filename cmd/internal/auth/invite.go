package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// inviteRequest creates a teammate in the calling admin's own org. No email
// is sent anywhere: the response carries the invite code and the admin
// shares the link out of band.
type inviteRequest struct {
	Email      string `json:"email"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	Role       string `json:"role"`
}

type inviteResponse struct {
	UserID     string `json:"userId"`
	InviteCode string `json:"inviteCode"`
}

// InviteHandler creates a passwordless user in the caller's org and returns
// an invite code. The caller must be authenticated; the org comes from
// their token, never from the request.
func (c *Config) InviteHandler() http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		caller, err := c.verifyBearer(r.Context(), r.Header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if !caller.IsAdmin() {
			http.Error(w, "only admins can invite teammates", http.StatusForbidden)
			return
		}
		var req inviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.GivenName == "" || req.FamilyName == "" {
			http.Error(w, "email, givenName, and familyName are required", http.StatusBadRequest)
			return
		}
		if req.Role != RoleAdmin && req.Role != RoleCreator && req.Role != RoleViewer {
			http.Error(w, "role must be admin, creator, or viewer", http.StatusBadRequest)
			return
		}

		user := map[string]any{
			"organization": map[string]any{"orgId": caller.OrgID},
			"profile":      map[string]any{"givenName": req.GivenName, "familyName": req.FamilyName},
			"email":        map[string]any{"email": req.Email},
		}
		var created struct {
			UserID string `json:"userId"`
		}
		if err := c.postZitadel(client, r, "/v2/users/human", user, &created); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var code struct {
			InviteCode string `json:"inviteCode"`
		}
		if err := c.postZitadel(client, r, "/v2/users/"+created.UserID+"/invite_code", map[string]any{"returnCode": map[string]any{}}, &code); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := c.grantRole(client, r, caller.OrgID, created.UserID, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(inviteResponse{UserID: created.UserID, InviteCode: code.InviteCode})
	})
}

// grantRole assigns one filament project role to a user in the given org.
// Orgs other than the project's own need the project granted to them first.
func (c *Config) grantRole(client *http.Client, r *http.Request, orgID, userID, role string) error {
	body := map[string]any{"projectId": c.projectID, "roleKeys": []string{role}}
	if orgID != c.ownerOrgID {
		grantID, err := c.ensureProjectGrant(client, r, orgID)
		if err != nil {
			return err
		}
		body["projectGrantId"] = grantID
	}
	return c.postZitadelOrg(client, r, orgID, "/management/v1/users/"+userID+"/grants", body, &struct{}{})
}

// ensureProjectGrant shares the filament project (all roles) with an org,
// once, and returns the grant id user grants must reference.
func (c *Config) ensureProjectGrant(client *http.Client, r *http.Request, orgID string) (string, error) {
	if id, ok := c.projectGrants.Load(orgID); ok {
		return id.(string), nil
	}
	var out struct {
		GrantID string `json:"grantId"`
	}
	create := map[string]any{"grantedOrgId": orgID, "roleKeys": projectRoles}
	if err := c.postZitadelOrg(client, r, c.ownerOrgID, "/management/v1/projects/"+c.projectID+"/grants", create, &out); err == nil {
		c.projectGrants.Store(orgID, out.GrantID)
		return out.GrantID, nil
	}
	// Already granted (e.g. a previous boot); look it up.
	var found struct {
		Result []struct {
			GrantID      string `json:"grantId"`
			GrantedOrgID string `json:"grantedOrgId"`
		} `json:"result"`
	}
	if err := c.postZitadelOrg(client, r, c.ownerOrgID, "/management/v1/projects/"+c.projectID+"/grants/_search", map[string]any{}, &found); err != nil {
		return "", err
	}
	for _, g := range found.Result {
		if g.GrantedOrgID == orgID {
			c.projectGrants.Store(orgID, g.GrantID)
			return g.GrantID, nil
		}
	}
	return "", fmt.Errorf("project grant for org %s not found", orgID)
}

// acceptRequest redeems an invite: verify the code, set the password. The
// user signs in through the normal flow afterwards.
type acceptRequest struct {
	UserID   string `json:"userId"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// InviteAcceptHandler is unauthenticated by design: the code is the
// credential.
func (c *Config) InviteAcceptHandler() http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req acceptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Code == "" || req.Password == "" {
			http.Error(w, "userId, code, and password are required", http.StatusBadRequest)
			return
		}
		if err := c.postZitadel(client, r, "/v2/users/"+req.UserID+"/invite_code/verify", map[string]any{"verificationCode": req.Code}, &struct{}{}); err != nil {
			http.Error(w, "invalid or expired invite", http.StatusUnauthorized)
			return
		}
		password := map[string]any{
			"newPassword": map[string]any{"password": req.Password, "changeRequired": false},
		}
		if err := c.postZitadel(client, r, "/v2/users/"+req.UserID+"/password", password, &struct{}{}); err != nil {
			// Password policy failures land here; surface Zitadel's reason.
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
