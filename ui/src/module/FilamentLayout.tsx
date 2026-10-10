import { type FC, type ReactNode, useCallback } from "react";

import { Outlet } from "@tanstack/react-router";

import MainLayout from "@/layouts/main/MainLayout";

import CreateConnectionModal from "@/pages/connections/components/create/CreateConnectionModal";
import ConnectionDrawer from "@/pages/connections/components/drawer/ConnectionDrawer";
import EditConnectionModal from "@/pages/connections/components/edit/EditConnectionModal";
import CreatePipelineModal from "@/pages/pipelines/components/create/CreatePipelineModal";

import { useFilamentLayoutSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { Flow } from "@/module/types";

export interface FilamentLayoutProps {
  sidebarFooter?: ReactNode;
}

const FilamentLayout: FC<FilamentLayoutProps> = ({ sidebarFooter }) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const { connectionId, flow } = useFilamentLayoutSearch();

  const handleCloseDrawer = useCallback(() => {
    void updateSearch(
      (prev) => {
        const {
          connectionId: _,
          connector: __,
          connectorKind: ___,
          flow: prevFlow,
          ...rest
        } = prev;
        return prevFlow === Flow.EDIT_CONNECTION ? rest : { ...rest, flow: prevFlow };
      },
      { replace: true },
    );
  }, [updateSearch]);

  const handleCloseFlow = useCallback(() => {
    void updateSearch(
      (prev) => {
        const { flow: _, connector: __, connectorKind: ___, connectorSearch: ____, ...rest } = prev;
        return rest;
      },
      { replace: true },
    );
  }, [updateSearch]);

  return (
    <>
      <MainLayout sidebarFooter={sidebarFooter}>
        <Outlet />
      </MainLayout>
      <ConnectionDrawer
        connectionId={connectionId}
        isOpen={!!connectionId}
        onClose={handleCloseDrawer}
      />
      {flow === Flow.CREATE_CONNECTION && <CreateConnectionModal onClose={handleCloseFlow} />}
      {flow === Flow.EDIT_CONNECTION && !!connectionId && (
        <EditConnectionModal onClose={handleCloseFlow} />
      )}
      {flow === Flow.CREATE_PIPELINE && <CreatePipelineModal onClose={handleCloseFlow} />}
    </>
  );
};

export default FilamentLayout;
