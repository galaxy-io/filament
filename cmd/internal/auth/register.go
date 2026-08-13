package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

// registerRequest creates an organization and its first admin: filament's
// self-service signup. The org becomes the tenant on first authenticated
// request.
type registerRequest struct {
	OrgName    string `json:"orgName"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

// RegisterHandler proxies signup to Zitadel's org API: one call creates the
// organization with a human admin. The email is marked verified because the
// dev instance sends no mail; the user signs in through the normal flow
// afterwards.
func (c *Config) RegisterHandler() http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			req.OrgName == "" || req.GivenName == "" || req.FamilyName == "" || req.Email == "" || req.Password == "" {
			http.Error(w, "orgName, givenName, familyName, email, and password are required", http.StatusBadRequest)
			return
		}

		org := map[string]any{
			"name": req.OrgName,
			"admins": []any{map[string]any{
				"human": map[string]any{
					"profile":  map[string]any{"givenName": req.GivenName, "familyName": req.FamilyName},
					"email":    map[string]any{"email": req.Email, "isVerified": true},
					"password": map[string]any{"password": req.Password},
				},
			}},
		}
		var out struct {
			OrganizationID string `json:"organizationId"`
			CreatedAdmins  []struct {
				UserID string `json:"userId"`
			} `json:"createdAdmins"`
		}
		if err := c.postZitadel(client, r, "/v2/organizations", org, &out); err != nil {
			// Duplicate org names and password-policy failures both land
			// here; the message carries Zitadel's reason.
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// The founder is the org's admin.
		for _, admin := range out.CreatedAdmins {
			if err := c.grantRole(client, r, out.OrganizationID, admin.UserID, RoleAdmin); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"organizationId": out.OrganizationID})
	})
}
