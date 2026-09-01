import { type User, UserManager } from "oidc-client-ts";

import type { GetAuthConfigResponse } from "@/gen/auth/v1/session_pb";

import { AUTH_CALLBACK_PATH, OIDC_SCOPE } from "@/auth/constants";

let userManager: UserManager | null = null;
let currentUser: User | null = null;
let signinCallbackResult: Promise<User | undefined> | null = null;
const subscribers = new Set<() => void>();

const setCurrentUser = (user: User | null) => {
  currentUser = user;
  for (const subscriber of subscribers) {
    subscriber();
  }
};

const requireUserManager = (): UserManager => {
  if (!userManager) {
    throw new Error("OIDC has not been initialized");
  }
  return userManager;
};

export const initOidc = (config: GetAuthConfigResponse) => {
  if (userManager) {
    return;
  }
  userManager = new UserManager({
    authority: config.issuer,
    client_id: config.clientId,
    redirect_uri: `${window.location.origin}${AUTH_CALLBACK_PATH}`,
    post_logout_redirect_uri: window.location.origin,
    scope: OIDC_SCOPE,
    automaticSilentRenew: true,
  });
  userManager.events.addUserLoaded((user) => setCurrentUser(user));
  userManager.events.addUserUnloaded(() => setCurrentUser(null));
  userManager.events.addSilentRenewError(() => {
    void redirectToSignIn(window.location.pathname + window.location.search);
  });
};

export const getAccessToken = (): string | undefined => currentUser?.access_token;

export const getSessionUser = (): User | null => currentUser;

export const subscribeToSession = (onChange: () => void): (() => void) => {
  subscribers.add(onChange);
  return () => {
    subscribers.delete(onChange);
  };
};

export const ensureSession = async (): Promise<User | null> => {
  const user = await requireUserManager().getUser();
  if (!user || user.expired) {
    return null;
  }
  setCurrentUser(user);
  return user;
};

export const redirectToSignIn = async (returnTo?: string): Promise<never> => {
  await requireUserManager().signinRedirect({ state: { returnTo } });
  return new Promise<never>(() => {});
};

export const completeSignIn = (): Promise<User | undefined> => {
  signinCallbackResult ??= requireUserManager().signinCallback();
  return signinCallbackResult;
};

export const redirectToSignOut = async (): Promise<never> => {
  await requireUserManager().signoutRedirect();
  return new Promise<never>(() => {});
};
