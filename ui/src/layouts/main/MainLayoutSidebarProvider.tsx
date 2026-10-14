import { createContext, type FC, type PropsWithChildren, useContext } from "react";

const MainLayoutSidebarCollapsedContext = createContext(false);

interface MainLayoutSidebarProviderProps {
  isCollapsed: boolean;
}

export const MainLayoutSidebarProvider: FC<PropsWithChildren<MainLayoutSidebarProviderProps>> = ({
  isCollapsed,
  children,
}) => (
  <MainLayoutSidebarCollapsedContext.Provider value={isCollapsed}>
    {children}
  </MainLayoutSidebarCollapsedContext.Provider>
);

export const useIsMainLayoutSidebarCollapsed = (): boolean =>
  useContext(MainLayoutSidebarCollapsedContext);
