package auth

import "errors"

// Sentinel errors separate "no credentials" from a damaged store or an
// authentication server problem, so the CLI never tells a user to log in
// when logging in would not help. Producers wrap them with %w; callers
// match with errors.Is. The text is written for a terminal: the CLI prints
// it as-is, so it names the server and the client id, never OIDC terms.
var (
	// ErrProfileNotFound: no stored profile under that name. Login fixes it.
	ErrProfileNotFound = errors.New("no stored credentials")
	// ErrProfileInvalid: a profile lacks a field needed to mint a token.
	ErrProfileInvalid = errors.New("credentials are incomplete")
	// ErrCredentialsCorrupt: the credentials file exists but does not parse.
	ErrCredentialsCorrupt = errors.New("credentials file is damaged")
	// ErrAuthServerUnreachable: the authentication server gave no answer.
	ErrAuthServerUnreachable = errors.New("authentication server is unreachable")
	// ErrAuthServerInvalid: the authentication server answered with something unusable.
	ErrAuthServerInvalid = errors.New("authentication server gave an unusable response")
	// ErrTokenRejected: the authentication server refused the client id and secret.
	ErrTokenRejected = errors.New("client id or secret was not accepted")
)
