import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { createRootRoute, Outlet, useNavigate, useSearch } from "@tanstack/react-router";
import { z } from "zod";

import Drawer from "@galaxy-io/dls/drawer/Drawer";
import Modal from "@galaxy-io/dls/modal/Modal";
import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import Text from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectionDrawer } from "@/pages/connectors/components/drawer";
import { CONNECTOR_DRAWER_WIDTH } from "@/pages/connectors/constants";

import { ToastProvider } from "@/providers/toast/ToastProvider";

import { useListConnectionsQuery } from "@/api/queries/connectors";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

export enum Flow {
  CREATE_CONNECTOR = "CREATE_CONNECTOR",
}

const rootSearchSchema = z.object({
  connectionId: z.string().optional(),
  flow: z.nativeEnum(Flow).optional(),
});

export const Route = createRootRoute({
  component: RootComponent,
  validateSearch: rootSearchSchema,
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

const ModalWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 500px;
  background-color: ${({ theme }) => theme.color.background.base};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  padding: 24px;
`);

function RootComponent() {
  const navigate = useNavigate();
  const { connectionId, flow } = useSearch({ from: "__root__" });

  const { data } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.UNSPECIFIED }),
    options: {
      enabled: !!connectionId,
    },
  });

  const selectedConnection = useMemo(() => {
    if (!connectionId || !data?.connections) return null;
    return data.connections.find((c) => c.id === connectionId) ?? null;
  }, [connectionId, data?.connections]);

  const handleCloseDrawer = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { connectionId: _, ...rest } = prev;
        return rest;
      },
    });
  };

  const handleCloseFlow = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { flow: _, ...rest } = prev;
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

        <Drawer
          open={!!selectedConnection}
          onClose={handleCloseDrawer}
          width={CONNECTOR_DRAWER_WIDTH}
        >
          {selectedConnection && (
            <ConnectionDrawer connection={selectedConnection} onClose={handleCloseDrawer} />
          )}
        </Drawer>

        <Modal open={flow === Flow.CREATE_CONNECTOR} onClose={handleCloseFlow}>
          <ModalWrapper>
            <Text>
              Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
              incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud
              exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
            </Text>
          </ModalWrapper>
        </Modal>
      </OverlayProvider>
    </ToastProvider>
  );
}
