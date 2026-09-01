import { UserManager } from "oidc-client-ts";

import type { GetAuthConfigResponse } from "@/gen/auth/v1/session_pb";

import { AUTH_CALLBACK_PATH, OIDC_SCOPE } from "@/auth/constants";

let userManager: UserManager | null = null;

export const initOidc = (config: GetAuthConfigResponse): UserManager => {
  if (userManager) {
    return userManager;
  }
  const manager = new UserManager({
    authority: config.issuer,
    client_id: config.clientId,
    redirect_uri: `${window.location.origin}${AUTH_CALLBACK_PATH}`,
    post_logout_redirect_uri: window.location.origin,
    scope: OIDC_SCOPE,
    automaticSilentRenew: true,
  });
  manager.events.addSilentRenewError(() => void manager.removeUser());
  userManager = manager;
  return manager;
};

export const getAccessToken = async (): Promise<string | undefined> =>
  (await userManager?.getUser())?.access_token;
