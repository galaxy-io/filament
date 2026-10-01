package identity

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// InviteURL builds the link that redeems an invitation on the UI, which
// decodes the user id and code back out of the path. Providers hand it to
// whoever delivers the invitation, since filament sends no mail.
func InviteURL(uiOrigin, userID, code string) string {
	token, _ := json.Marshal(struct {
		UserID string `json:"userId"`
		Code   string `json:"code"`
	}{userID, code})
	return strings.TrimRight(uiOrigin, "/") + "/invite/" + base64.RawURLEncoding.EncodeToString(token)
}
