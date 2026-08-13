package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

// member is one user of the caller's org, shaped for the team dialog.
type member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	State string `json:"state"`
	Role  string `json:"role"`
}

// MembersHandler lists the users of the caller's org with their filament
// role. The org comes from the bearer token, so a caller can only ever see
// their own tenant.
func (c *Config) MembersHandler() http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		caller, err := c.verifyBearer(r.Context(), r.Header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		var users struct {
			Result []struct {
				UserID string `json:"userId"`
				State  string `json:"state"`
				Human  struct {
					Profile struct {
						GivenName   string `json:"givenName"`
						FamilyName  string `json:"familyName"`
						DisplayName string `json:"displayName"`
					} `json:"profile"`
					Email struct {
						Email string `json:"email"`
					} `json:"email"`
				} `json:"human"`
			} `json:"result"`
		}
		search := map[string]any{
			"queries": []any{
				map[string]any{"organizationIdQuery": map[string]any{"organizationId": caller.OrgID}},
				map[string]any{"typeQuery": map[string]any{"type": "TYPE_HUMAN"}},
			},
		}
		if err := c.postZitadel(client, r, "/v2/users", search, &users); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		var grants struct {
			Result []struct {
				UserID   string   `json:"userId"`
				RoleKeys []string `json:"roleKeys"`
			} `json:"result"`
		}
		grantSearch := map[string]any{
			"queries": []any{map[string]any{"projectIdQuery": map[string]any{"projectId": c.projectID}}},
		}
		if err := c.postZitadelOrg(client, r, caller.OrgID, "/management/v1/users/grants/_search", grantSearch, &grants); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		roles := make(map[string]string, len(grants.Result))
		for _, g := range grants.Result {
			if len(g.RoleKeys) > 0 {
				roles[g.UserID] = g.RoleKeys[0]
			}
		}

		members := make([]member, 0, len(users.Result))
		for _, u := range users.Result {
			name := u.Human.Profile.DisplayName
			if name == "" {
				name = u.Human.Profile.GivenName + " " + u.Human.Profile.FamilyName
			}
			members = append(members, member{
				ID:    u.UserID,
				Name:  name,
				Email: u.Human.Email.Email,
				State: u.State,
				Role:  roles[u.UserID],
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"members": members, "canManage": caller.IsAdmin()})
	})
}
