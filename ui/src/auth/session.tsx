import { createContext, type PropsWithChildren, useContext } from "react";

export interface AppSession {
  isAuthEnabled: boolean;
  accessToken?: string;
  userId?: string;
  name?: string;
  email?: string;
  avatarUrl?: string;
}

const DEFAULT_SESSION: AppSession = {
  isAuthEnabled: false,
};

const AppSessionContext = createContext<AppSession>(DEFAULT_SESSION);

export const AppSessionProvider = ({
  value,
  children,
}: PropsWithChildren<{ value: AppSession }>) => {
  return <AppSessionContext.Provider value={value}>{children}</AppSessionContext.Provider>;
};

export const useAppSession = () => useContext(AppSessionContext);
