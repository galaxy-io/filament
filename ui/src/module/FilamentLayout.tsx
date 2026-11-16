import { type FC, type ReactNode, useCallback } from "react";

import { Outlet } from "@tanstack/react-router";

import MainLayout from "@/layouts/main/MainLayout";

import CreateConnectionModal from "@/pages/connections/components/create/CreateConnectionModal";
import ConnectionDrawer from "@/pages/connections/components/drawer/ConnectionDrawer";
import EditConnectionModal from "@/pages/connections/components/edit/EditConnectionModal";
import CreatePipelineModal from "@/pages/pipelines/components/create/CreatePipelineModal";
import PipelineRunModal from "@/pages/pipelines/components/run/PipelineRunModal";

import { FILAMENT_LAYOUT_DEFAULT_PRODUCT_URLS } from "@/module/constants";
import { GalaxyProductsProvider } from "@/module/GalaxyProductsProvider";
import { useFilamentLayoutSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { Flow, type GalaxyProductUrls } from "@/module/types";
import { createFilamentFlowCloseSearch } from "@/module/utils";

export interface FilamentLayoutProps {
  sidebarFooter?: ReactNode;
  productUrls?: GalaxyProductUrls;
}

const FilamentLayout: FC<FilamentLayoutProps> = ({
  sidebarFooter,
  productUrls = FILAMENT_LAYOUT_DEFAULT_PRODUCT_URLS,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const {
    connectionId,
    pipelineId,
    flow,
    connector,
    connectorKind,
    connectorSearch,
    sourceId,
    sinkIds,
    resources,
    resyncResources,
  } = useFilamentLayoutSearch();

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
    void updateSearch((prev) => ({ ...prev, ...createFilamentFlowCloseSearch(prev) }), {
      replace: true,
    });
  }, [updateSearch]);

  return (
    <GalaxyProductsProvider productUrls={productUrls}>
      <MainLayout sidebarFooter={sidebarFooter}>
        <Outlet />
      </MainLayout>
      <ConnectionDrawer
        connectionId={connectionId}
        isOpen={!!connectionId}
        onClose={handleCloseDrawer}
      />
      <CreateConnectionModal
        isOpen={flow === Flow.CREATE_CONNECTION}
        connector={connector}
        connectorKind={connectorKind}
        connectorSearch={connectorSearch}
        onClose={handleCloseFlow}
      />
      <EditConnectionModal
        isOpen={flow === Flow.EDIT_CONNECTION && !!connectionId}
        connectionId={connectionId}
        onClose={handleCloseFlow}
      />
      <CreatePipelineModal
        isOpen={flow === Flow.CREATE_PIPELINE}
        sourceId={sourceId}
        sinkIds={sinkIds}
        onClose={handleCloseFlow}
      />
      <PipelineRunModal
        isOpen={flow === Flow.RUN_PIPELINE && !!pipelineId}
        pipelineId={pipelineId}
        resources={resources}
        resyncResources={resyncResources}
        onClose={handleCloseFlow}
      />
    </GalaxyProductsProvider>
  );
};

export default FilamentLayout;
