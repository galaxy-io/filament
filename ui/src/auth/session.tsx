import { createContext, type PropsWithChildren, useContext } from "react";

import type { AppSession } from "@/auth/types";

interface AppSessionProviderProps {
  value: AppSession;
}

const DEFAULT_SESSION: AppSession = {
  isAuthenticated: false,
};

const AppSessionContext = createContext<AppSession>(DEFAULT_SESSION);

export const AppSessionProvider = ({
  value,
  children,
}: PropsWithChildren<AppSessionProviderProps>) => {
  return <AppSessionContext.Provider value={value}>{children}</AppSessionContext.Provider>;
};

export const useAppSession = () => useContext(AppSessionContext);
