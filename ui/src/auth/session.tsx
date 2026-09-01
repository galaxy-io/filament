import { createContext, type PropsWithChildren, useContext, useMemo } from "react";

import { useAuth } from "react-oidc-context";

import type { AppSession } from "@/auth/types";

const DEFAULT_SESSION: AppSession = {
  isAuthenticated: false,
};

const AppSessionContext = createContext<AppSession>(DEFAULT_SESSION);

export const AppSessionProvider = ({ children }: PropsWithChildren) => {
  const { user, isAuthenticated } = useAuth();

  const session = useMemo<AppSession>(
    () =>
      isAuthenticated && user
        ? {
            isAuthenticated: true,
            userId: user.profile.sub,
            name: user.profile.name,
            email: user.profile.email,
            avatarUrl: user.profile.picture,
          }
        : DEFAULT_SESSION,
    [isAuthenticated, user],
  );

  return <AppSessionContext.Provider value={session}>{children}</AppSessionContext.Provider>;
};

export const useAppSession = () => useContext(AppSessionContext);
