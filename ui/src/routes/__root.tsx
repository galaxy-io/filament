import { styled } from "@linaria/react";
import { createRootRoute, Outlet, useNavigate, useSearch } from "@tanstack/react-router";
import { z } from "zod";

import Drawer from "@galaxy-io/dls/drawer/Drawer";
import Modal from "@galaxy-io/dls/modal/Modal";
import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastProvider } from "@galaxy-io/dls/toast/ToastProvider";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionModal from "@/pages/connectors/components/create/CreateConnectionModal";
import ConnectionDrawer from "@/pages/connectors/components/drawer/ConnectionDrawer";
import EditConnectionModal from "@/pages/connectors/components/edit/EditConnectionModal";
import { CONNECTOR_DRAWER_WIDTH } from "@/pages/connectors/constants";
import CreatePipelineModal from "@/pages/pipelines/components/create/CreatePipelineModal";

export enum Flow {
  CREATE_CONNECTION = "CREATE_CONNECTION",
  EDIT_CONNECTION = "EDIT_CONNECTION",
  CREATE_PIPELINE = "CREATE_PIPELINE",
}

const searchParams = z.object({
  connectionId: z.string().optional().catch(undefined),
  flow: z.enum(Flow).optional().catch(undefined),
  connectorKind: z.enum(ConnectorKind).optional().catch(undefined),
  connector: z.string().optional().catch(undefined),
});

export const Route = createRootRoute({
  component: RootComponent,
  validateSearch: searchParams,
});

const RootComponentWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;

  height: 100vh;
  width: 100vw;
  background-color: ${({ theme }) => theme.color.background.base};

  ::selection {
    background-color: ${({ theme }) => theme.color.background.blue};
  }
`);

function RootComponent() {
  const navigate = useNavigate();
  const { connectionId, flow } = useSearch({ from: "__root__" });

  const handleCloseDrawer = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { connectionId: _, flow: prevFlow, ...rest } = prev;
        return prevFlow === Flow.EDIT_CONNECTION ? rest : { ...rest, flow: prevFlow };
      },
    });
  };

  const handleCloseFlow = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { flow: _, connector: __, connectorKind: ___, ...rest } = prev;
        return rest;
      },
    });
  };

  return (
    <ToastProvider>
      <OverlayProvider>
        <RootComponentWrapper>
          <Outlet />
        </RootComponentWrapper>
        <Drawer open={!!connectionId} onClose={handleCloseDrawer} width={CONNECTOR_DRAWER_WIDTH}>
          {connectionId && <ConnectionDrawer onClose={handleCloseDrawer} />}
        </Drawer>
        <Modal open={flow === Flow.CREATE_CONNECTION} onClose={handleCloseFlow}>
          <CreateConnectionModal onClose={handleCloseFlow} />
        </Modal>
        <Modal open={flow === Flow.EDIT_CONNECTION && !!connectionId} onClose={handleCloseFlow}>
          {connectionId && <EditConnectionModal onClose={handleCloseFlow} />}
        </Modal>
        <Modal open={flow === Flow.CREATE_PIPELINE} onClose={handleCloseFlow}>
          <CreatePipelineModal onClose={handleCloseFlow} />
        </Modal>
      </OverlayProvider>
    </ToastProvider>
  );
}
