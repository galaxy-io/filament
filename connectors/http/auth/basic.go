package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("basic", newBasic) }

type basicAuth struct {
	user string
	pass string
}

func newBasic(params map[string]any) (Authenticator, error) {
	user, err := requireStrParam(params, "basic", "user")
	if err != nil {
		return nil, err
	}
	// Empty password is legitimate (e.g. Stripe API keys use the key as the
	// username with an empty password). Don't require it.
	pass := strParam(params, "pass")
	return &basicAuth{user: user, pass: pass}, nil
}

func (a *basicAuth) Apply(_ context.Context, req *http.Request, scope template.Scope) error {
	user, err := template.Render(a.user, scope)
	if err != nil {
		return err
	}
	pass, err := template.Render(a.pass, scope)
	if err != nil {
		return err
	}
	if err := validateBasicCreds(user, pass); err != nil {
		return err
	}
	req.SetBasicAuth(user, pass)
	return nil
}

// DeclaredWrites reports that basic auth uses the Authorization header.
func (a *basicAuth) DeclaredWrites() []Write {
	return []Write{{Kind: WriteHeader, Name: "Authorization"}}
}

// validateBasicCreds enforces RFC 7617's restrictions: user must not contain
// the ":" separator, and neither field may contain control characters that
// would corrupt the base64-encoded `user:pass` payload (CR/LF, NULL).
//
// Validation runs at Apply time because user/pass are usually templated and
// thus unknown until the request scope is resolved.
func validateBasicCreds(user, pass string) error {
	if strings.ContainsRune(user, ':') {
		return fmt.Errorf("basic auth: user must not contain ':' (use the userinfo form)")
	}
	if i := strings.IndexAny(user, "\x00\r\n"); i >= 0 {
		return fmt.Errorf("basic auth: user contains control character at byte %d", i)
	}
	if i := strings.IndexAny(pass, "\x00\r\n"); i >= 0 {
		return fmt.Errorf("basic auth: pass contains control character at byte %d", i)
	}
	return nil
}
