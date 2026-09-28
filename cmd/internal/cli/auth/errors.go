package auth

import "errors"

// Sentinel errors separate "no credentials" from a damaged store or an
// server problem, so the CLI never tells a user to log in when logging in
// would not help. Producers wrap them with %w; callers match with errors.Is.
// The text is written for a terminal: the CLI prints it as-is, so it names
// the server and the client id, never OIDC terms.
var (
	// ErrProfileNotFound: no stored profile under that name. Login fixes it.
	ErrProfileNotFound = errors.New("no stored credentials")
	// ErrProfileInvalid: a profile lacks a field needed to mint a token.
	ErrProfileInvalid = errors.New("credentials are incomplete")
	// ErrCredentialsCorrupt: the credentials file exists but does not parse.
	ErrCredentialsCorrupt = errors.New("credentials file is damaged")
	// ErrAuthServerUnreachable: the server could not mint a token.
	ErrAuthServerUnreachable = errors.New("server could not issue a token")
	// ErrTokenRejected: the server refused the client id and secret.
	ErrTokenRejected = errors.New("client id or secret was not accepted")
)
