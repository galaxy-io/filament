import { createContext, type FC, type PropsWithChildren, useContext } from "react";

const MainLayoutSidebarCollapsedContext = createContext(false);

interface MainLayoutSidebarProviderProps {
  isCollapsed: boolean;
}

/** Shares whether the sidebar is the icon rail with footer content passed in from outside it. */
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
