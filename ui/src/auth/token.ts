// Bridges the OIDC session to the rest of the app without coupling other
// modules to react-oidc-context.

export interface SessionProfile {
  // The Zitadel user id; always present, unlike name and email which need
  // a userinfo fetch.
  sub?: string;
  name?: string;
  email?: string;
}

let getter: (() => string | undefined) | null = null;
let profileGetter: (() => SessionProfile | undefined) | null = null;

export const setAccessTokenGetter = (get: (() => string | undefined) | null) => {
  getter = get;
};

export const getAccessToken = (): string | undefined => getter?.();

export const setProfileGetter = (get: (() => SessionProfile | undefined) | null) => {
  profileGetter = get;
};

export const getProfile = (): SessionProfile | undefined => profileGetter?.();
