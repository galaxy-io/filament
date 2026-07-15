import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import {
  createRootRoute,
  Outlet,
  useNavigate,
  useSearch,
} from "@tanstack/react-router";
import { z } from "zod";

import Drawer from "@galaxy-io/dls/drawer/Drawer";
import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectorsRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import { ConnectorDrawer } from "@/pages/connectors/components/drawer";
import { CONNECTOR_DRAWER_WIDTH } from "@/pages/connectors/constants";

const rootSearchSchema = z.object({
  connectorId: z.string().optional(),
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

function RootComponent() {
  const navigate = useNavigate();
  const { connectorId } = useSearch({ from: "__root__" });

  const { data } = useListConnectorsQuery({
    input: create(ListConnectorsRequestSchema, {
      kind: ConnectorKind.UNSPECIFIED,
    }),
    options: {
      enabled: !!connectorId,
    },
  });

  const selectedConnector = useMemo(() => {
    if (!connectorId || !data?.connectors) return null;
    return data.connectors.find((c) => c.name === connectorId) ?? null;
  }, [connectorId, data?.connectors]);

  const handleCloseDrawer = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { connectorId: _, ...rest } = prev;
        return rest;
      },
    });
  };

  return (
    <OverlayProvider>
      <RootComponentWrapper>
        <Outlet />
      </RootComponentWrapper>

      <Drawer
        open={!!selectedConnector}
        onClose={handleCloseDrawer}
        width={CONNECTOR_DRAWER_WIDTH}
      >
        {selectedConnector && (
          <ConnectorDrawer
            connector={selectedConnector}
            onClose={handleCloseDrawer}
          />
        )}
      </Drawer>
    </OverlayProvider>
  );
}
