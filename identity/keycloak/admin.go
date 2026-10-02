package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// admin is the slice of Keycloak's admin REST API filament uses, issued as
// the filament client's service account. Keycloak ships no Go SDK, so the
// representations here are the fields filament reads and writes.
type admin struct {
	base   string
	client *http.Client
}

// apiError is a non-2xx answer from Keycloak with the reason it gave.
type apiError struct {
	status  int
	message string
}

func (e *apiError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("keycloak: status %d", e.status)
	}
	return fmt.Sprintf("keycloak: %s", e.message)
}

// errUnavailable marks an error that means Keycloak could not be reached at
// all, which must never be mistaken for a rejected credential.
var errUnavailable = errors.New("keycloak unavailable")

func isStatus(err error, status int) bool {
	var e *apiError
	return errors.As(err, &e) && e.status == status
}

// do issues one request. in is JSON-encoded unless it is a string, which is
// sent verbatim; out receives a JSON body when non-nil. The Location header
// comes back so a create can read the new id off it.
func (a *admin) do(ctx context.Context, method, path string, in, out any) (string, error) {
	var body io.Reader
	switch v := in.(type) {
	case nil:
	case string:
		body = strings.NewReader(v)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errUnavailable, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", &apiError{status: res.StatusCode, message: reason(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return "", fmt.Errorf("keycloak: decode %s %s: %w", method, path, err)
		}
	}
	return res.Header.Get("Location"), nil
}

func decodeJSON(r io.Reader, out any) error { return json.NewDecoder(r).Decode(out) }

// reason pulls the message out of Keycloak's two error shapes.
func reason(raw []byte) string {
	var body struct {
		ErrorMessage     string `json:"errorMessage"`
		ErrorDescription string `json:"error_description"`
		Error            string `json:"error"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return strings.TrimSpace(string(raw))
	}
	switch {
	case body.ErrorMessage != "":
		return body.ErrorMessage
	case body.ErrorDescription != "":
		return body.ErrorDescription
	}
	return body.Error
}

// created returns the id a create answered with on its Location header.
func created(location string) string {
	return location[strings.LastIndex(location, "/")+1:]
}

type userRep struct {
	ID            string              `json:"id,omitempty"`
	Username      string              `json:"username,omitempty"`
	Email         string              `json:"email,omitempty"`
	FirstName     string              `json:"firstName,omitempty"`
	LastName      string              `json:"lastName,omitempty"`
	Enabled       bool                `json:"enabled"`
	EmailVerified bool                `json:"emailVerified"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
	Credentials   []credentialRep     `json:"credentials,omitempty"`
}

type credentialRep struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

type organizationRep struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

type roleRep struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

type clientRep struct {
	ID                        string              `json:"id,omitempty"`
	ClientID                  string              `json:"clientId"`
	Secret                    string              `json:"secret,omitempty"`
	Name                      string              `json:"name,omitempty"`
	Description               string              `json:"description,omitempty"`
	PublicClient              bool                `json:"publicClient"`
	ServiceAccountsEnabled    bool                `json:"serviceAccountsEnabled"`
	StandardFlowEnabled       bool                `json:"standardFlowEnabled"`
	DirectAccessGrantsEnabled bool                `json:"directAccessGrantsEnabled"`
	Attributes                map[string]string   `json:"attributes,omitempty"`
	ProtocolMappers           []protocolMapperRep `json:"protocolMappers,omitempty"`
}

type protocolMapperRep struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	Config         map[string]string `json:"config"`
}

// audienceMapper puts filament's client on a token's audience, which the
// verifier requires.
func audienceMapper(clientID string) protocolMapperRep {
	return protocolMapperRep{
		Name:           "filament audience",
		Protocol:       "openid-connect",
		ProtocolMapper: "oidc-audience-mapper",
		Config: map[string]string{
			"included.client.audience": clientID,
			"access.token.claim":       "true",
			"id.token.claim":           "false",
		},
	}
}

func (a *admin) getUser(ctx context.Context, id string) (userRep, error) {
	var user userRep
	_, err := a.do(ctx, http.MethodGet, "/users/"+url.PathEscape(id), nil, &user)
	return user, err
}

// getUserRaw reads a user as Keycloak represents it, for a read-modify-write:
// an update clears every field it omits.
func (a *admin) getUserRaw(ctx context.Context, id string) (map[string]any, error) {
	var user map[string]any
	_, err := a.do(ctx, http.MethodGet, "/users/"+url.PathEscape(id), nil, &user)
	return user, err
}

func (a *admin) putUser(ctx context.Context, id string, user map[string]any) error {
	_, err := a.do(ctx, http.MethodPut, "/users/"+url.PathEscape(id), user, nil)
	return err
}

func (a *admin) createUser(ctx context.Context, user userRep) (string, error) {
	location, err := a.do(ctx, http.MethodPost, "/users", user, nil)
	if err != nil {
		return "", err
	}
	return created(location), nil
}

// userByEmail resolves a user by their exact address.
func (a *admin) userByEmail(ctx context.Context, email string) (userRep, error) {
	var found []userRep
	if _, err := a.do(ctx, http.MethodGet, "/users?exact=true&email="+url.QueryEscape(email), nil, &found); err != nil {
		return userRep{}, err
	}
	if len(found) == 0 {
		return userRep{}, &apiError{status: http.StatusNotFound, message: fmt.Sprintf("user %q not found", email)}
	}
	return found[0], nil
}

func (a *admin) deleteUser(ctx context.Context, id string) error {
	_, err := a.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(id), nil, nil)
	return err
}

func (a *admin) resetPassword(ctx context.Context, id, password string) error {
	_, err := a.do(ctx, http.MethodPut, "/users/"+url.PathEscape(id)+"/reset-password",
		credentialRep{Type: "password", Value: password}, nil)
	return err
}

func (a *admin) createOrganization(ctx context.Context, org organizationRep) (string, error) {
	location, err := a.do(ctx, http.MethodPost, "/organizations", org, nil)
	if err != nil {
		return "", err
	}
	return created(location), nil
}

func (a *admin) deleteOrganization(ctx context.Context, id string) error {
	_, err := a.do(ctx, http.MethodDelete, "/organizations/"+url.PathEscape(id), nil, nil)
	return err
}

// organizationByAlias resolves the organization behind a token's alias.
func (a *admin) organizationByAlias(ctx context.Context, alias string) (organizationRep, error) {
	var found []organizationRep
	if _, err := a.do(ctx, http.MethodGet, "/organizations?q="+url.QueryEscape("alias:"+alias), nil, &found); err != nil {
		return organizationRep{}, err
	}
	for _, org := range found {
		if org.Alias == alias {
			return org, nil
		}
	}
	return organizationRep{}, &apiError{status: http.StatusNotFound, message: fmt.Sprintf("organization %q not found", alias)}
}

func (a *admin) addMember(ctx context.Context, orgID, userID string) error {
	_, err := a.do(ctx, http.MethodPost, "/organizations/"+url.PathEscape(orgID)+"/members", userID, nil)
	return err
}

func (a *admin) members(ctx context.Context, orgID string) ([]userRep, error) {
	var found []userRep
	_, err := a.do(ctx, http.MethodGet, "/organizations/"+url.PathEscape(orgID)+"/members?max=1000", nil, &found)
	return found, err
}

func (a *admin) memberOrganizations(ctx context.Context, userID string) ([]organizationRep, error) {
	var found []organizationRep
	_, err := a.do(ctx, http.MethodGet, "/organizations/members/"+url.PathEscape(userID)+"/organizations", nil, &found)
	return found, err
}

// clientByClientID resolves a client's uuid from the id tokens name it by.
func (a *admin) clientByClientID(ctx context.Context, clientID string) (clientRep, error) {
	var found []clientRep
	if _, err := a.do(ctx, http.MethodGet, "/clients?clientId="+url.QueryEscape(clientID), nil, &found); err != nil {
		return clientRep{}, err
	}
	for _, client := range found {
		if client.ClientID == clientID {
			return client, nil
		}
	}
	return clientRep{}, &apiError{status: http.StatusNotFound, message: fmt.Sprintf("client %q not found", clientID)}
}

// clients lists every client in the realm. Keycloak cannot search on
// attributes, so callers filter the list themselves.
func (a *admin) clients(ctx context.Context) ([]clientRep, error) {
	var found []clientRep
	_, err := a.do(ctx, http.MethodGet, "/clients?max=1000", nil, &found)
	return found, err
}

// getClientRaw reads a client as Keycloak represents it, for a
// read-modify-write.
func (a *admin) getClientRaw(ctx context.Context, id string) (map[string]any, error) {
	var client map[string]any
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(id), nil, &client)
	return client, err
}

func (a *admin) putClient(ctx context.Context, id string, client map[string]any) error {
	_, err := a.do(ctx, http.MethodPut, "/clients/"+url.PathEscape(id), client, nil)
	return err
}

func (a *admin) createClient(ctx context.Context, client clientRep) (string, error) {
	location, err := a.do(ctx, http.MethodPost, "/clients", client, nil)
	if err != nil {
		return "", err
	}
	return created(location), nil
}

func (a *admin) deleteClient(ctx context.Context, id string) error {
	_, err := a.do(ctx, http.MethodDelete, "/clients/"+url.PathEscape(id), nil, nil)
	return err
}

func (a *admin) serviceAccountUser(ctx context.Context, clientID string) (userRep, error) {
	var user userRep
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(clientID)+"/service-account-user", nil, &user)
	return user, err
}

func (a *admin) clientSecret(ctx context.Context, clientID string) (string, error) {
	var secret struct {
		Value string `json:"value"`
	}
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(clientID)+"/client-secret", nil, &secret)
	return secret.Value, err
}

func (a *admin) regenerateClientSecret(ctx context.Context, clientID string) (string, error) {
	var secret struct {
		Value string `json:"value"`
	}
	_, err := a.do(ctx, http.MethodPost, "/clients/"+url.PathEscape(clientID)+"/client-secret", nil, &secret)
	return secret.Value, err
}

func (a *admin) clientRoles(ctx context.Context, clientID string) ([]roleRep, error) {
	var found []roleRep
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(clientID)+"/roles", nil, &found)
	return found, err
}

func (a *admin) createClientRole(ctx context.Context, clientID, name string) error {
	_, err := a.do(ctx, http.MethodPost, "/clients/"+url.PathEscape(clientID)+"/roles", roleRep{Name: name}, nil)
	return err
}

// roleUsers lists every user holding one client role, across the realm.
func (a *admin) roleUsers(ctx context.Context, clientID, role string) ([]userRep, error) {
	var found []userRep
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(clientID)+"/roles/"+url.PathEscape(role)+"/users?max=1000", nil, &found)
	return found, err
}

func (a *admin) userClientRoles(ctx context.Context, userID, clientID string) ([]roleRep, error) {
	var found []roleRep
	_, err := a.do(ctx, http.MethodGet, "/users/"+url.PathEscape(userID)+"/role-mappings/clients/"+url.PathEscape(clientID), nil, &found)
	return found, err
}

func (a *admin) addUserClientRoles(ctx context.Context, userID, clientID string, roles []roleRep) error {
	_, err := a.do(ctx, http.MethodPost, "/users/"+url.PathEscape(userID)+"/role-mappings/clients/"+url.PathEscape(clientID), roles, nil)
	return err
}

func (a *admin) removeUserClientRoles(ctx context.Context, userID, clientID string, roles []roleRep) error {
	if len(roles) == 0 {
		return nil
	}
	_, err := a.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(userID)+"/role-mappings/clients/"+url.PathEscape(clientID), roles, nil)
	return err
}

func (a *admin) protocolMappers(ctx context.Context, clientID string) ([]protocolMapperRep, error) {
	var found []protocolMapperRep
	_, err := a.do(ctx, http.MethodGet, "/clients/"+url.PathEscape(clientID)+"/protocol-mappers/models", nil, &found)
	return found, err
}

func (a *admin) addProtocolMapper(ctx context.Context, clientID string, mapper protocolMapperRep) error {
	_, err := a.do(ctx, http.MethodPost, "/clients/"+url.PathEscape(clientID)+"/protocol-mappers/models", mapper, nil)
	return err
}

func (a *admin) realm(ctx context.Context) (map[string]any, error) {
	var realm map[string]any
	_, err := a.do(ctx, http.MethodGet, "", nil, &realm)
	return realm, err
}

func (a *admin) updateRealm(ctx context.Context, fields map[string]any) error {
	_, err := a.do(ctx, http.MethodPut, "", fields, nil)
	return err
}

func (a *admin) userProfile(ctx context.Context) (map[string]any, error) {
	var profile map[string]any
	_, err := a.do(ctx, http.MethodGet, "/users/profile", nil, &profile)
	return profile, err
}

func (a *admin) updateUserProfile(ctx context.Context, profile map[string]any) error {
	_, err := a.do(ctx, http.MethodPut, "/users/profile", profile, nil)
	return err
}
