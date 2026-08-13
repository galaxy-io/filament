package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// roleRequest reassigns a member's filament role.
type roleRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

// RoleHandler changes a member's role. The grant lookup runs in the
// caller's org, so members of other tenants are unreachable by
// construction.
func (c *Config) RoleHandler() http.Handler {
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
			http.Error(w, "only admins can change roles", http.StatusForbidden)
			return
		}
		var req roleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
			http.Error(w, "userId and role are required", http.StatusBadRequest)
			return
		}
		if req.Role != RoleAdmin && req.Role != RoleCreator && req.Role != RoleViewer {
			http.Error(w, "role must be admin, creator, or viewer", http.StatusBadRequest)
			return
		}

		grantID, err := c.findUserGrant(client, r, caller.OrgID, req.UserID)
		if err != nil {
			// No grant yet (e.g. user predates roles); create one instead.
			if err := c.grantRole(client, r, caller.OrgID, req.UserID, req.Role); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		update := map[string]any{"roleKeys": []string{req.Role}}
		if err := c.putZitadelOrg(client, r, caller.OrgID, "/management/v1/users/"+req.UserID+"/grants/"+grantID, update); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// removeRequest deletes a member from the caller's org.
type removeRequest struct {
	UserID string `json:"userId"`
}

// RemoveHandler deletes a member. The user must belong to the caller's org,
// and callers cannot remove themselves.
func (c *Config) RemoveHandler() http.Handler {
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
			http.Error(w, "only admins can remove members", http.StatusForbidden)
			return
		}
		var req removeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
			http.Error(w, "userId is required", http.StatusBadRequest)
			return
		}
		if req.UserID == caller.UserID {
			http.Error(w, "you cannot remove yourself", http.StatusBadRequest)
			return
		}

		// Membership check: the target must be in the caller's org.
		var found struct {
			Result []struct {
				UserID string `json:"userId"`
			} `json:"result"`
		}
		search := map[string]any{
			"queries": []any{
				map[string]any{"organizationIdQuery": map[string]any{"organizationId": caller.OrgID}},
				map[string]any{"inUserIdsQuery": map[string]any{"userIds": []string{req.UserID}}},
			},
		}
		if err := c.postZitadel(client, r, "/v2/users", search, &found); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if len(found.Result) == 0 {
			http.Error(w, "member not found", http.StatusNotFound)
			return
		}
		if err := c.deleteZitadel(client, r, "/v2/users/"+req.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// findUserGrant returns the id of the user's filament grant in the org.
func (c *Config) findUserGrant(client *http.Client, r *http.Request, orgID, userID string) (string, error) {
	var out struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	search := map[string]any{
		"queries": []any{
			map[string]any{"projectIdQuery": map[string]any{"projectId": c.projectID}},
			map[string]any{"userIdQuery": map[string]any{"userId": userID}},
		},
	}
	if err := c.postZitadelOrg(client, r, orgID, "/management/v1/users/grants/_search", search, &out); err != nil {
		return "", err
	}
	if len(out.Result) == 0 {
		return "", fmt.Errorf("member has no role grant")
	}
	return out.Result[0].ID, nil
}
