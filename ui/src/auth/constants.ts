export const AUTH_CALLBACK_PATH = "/auth/callback";
export const INVITE_PATH_PREFIX = "/invite";

const OIDC_BASE_SCOPES = ["openid", "profile", "email"];
const OIDC_OFFLINE_ACCESS_SCOPE = "offline_access";
const ZITADEL_ORG_SCOPE = "urn:zitadel:iam:user:resourceowner";

export const OIDC_SCOPE = [...OIDC_BASE_SCOPES, OIDC_OFFLINE_ACCESS_SCOPE, ZITADEL_ORG_SCOPE].join(
  " ",
);
