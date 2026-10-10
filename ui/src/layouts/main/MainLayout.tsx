import type { FC, PropsWithChildren, ReactNode } from "react";

import AppFrame from "@galaxy-io/dls/layout/AppFrame";

import MainLayoutSidebar from "@/layouts/main/MainLayoutSidebar";

interface MainLayoutProps {
  sidebarFooter?: ReactNode;
}

const MainLayout: FC<PropsWithChildren<MainLayoutProps>> = ({ sidebarFooter, children }) => (
  <AppFrame sidebar={<MainLayoutSidebar footer={sidebarFooter} />}>{children}</AppFrame>
);

export default MainLayout;
