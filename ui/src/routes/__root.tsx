import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { createRootRoute, Outlet, useNavigate, useSearch } from "@tanstack/react-router";
import { z } from "zod";

import Drawer from "@galaxy-io/dls/drawer/Drawer";
import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import { ListProvidersRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useListProvidersQuery } from "@/api/queries/providers";

import { ProviderDrawer } from "@/pages/providers/components/drawer";
import { PROVIDER_DRAWER_WIDTH } from "@/pages/providers/constants";

const rootSearchSchema = z.object({
  providerId: z.string().optional(),
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
  const { providerId } = useSearch({ from: "__root__" });

  const { data } = useListProvidersQuery({
    input: create(ListProvidersRequestSchema, { kind: ProviderKind.UNSPECIFIED }),
    options: {
      enabled: !!providerId,
    },
  });

  const selectedProvider = useMemo(() => {
    if (!providerId || !data?.providers) return null;
    return data.providers.find((p) => p.name === providerId) ?? null;
  }, [providerId, data?.providers]);

  const handleCloseDrawer = () => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { providerId: _, ...rest } = prev;
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
        open={!!selectedProvider}
        onClose={handleCloseDrawer}
        width={PROVIDER_DRAWER_WIDTH}
      >
        {selectedProvider && (
          <ProviderDrawer
            provider={selectedProvider}
            onClose={handleCloseDrawer}
          />
        )}
      </Drawer>
    </OverlayProvider>
  );
}
