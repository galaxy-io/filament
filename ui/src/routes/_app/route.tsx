import { Code, ConnectError } from "@connectrpc/connect";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { z } from "zod";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import AppLayout from "@/layouts/app/AppLayout";
import { Flow } from "@/layouts/app/types";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { createGetSessionQueryOptions } from "@/api/queries/auth";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

import { DEFAULT_SESSION } from "@/auth/constants";
import { sessionFromResponse } from "@/auth/utils";

const searchParams = z.object({
  connectionId: z.string().optional().catch(undefined),
  flow: z.enum(Flow).optional().catch(undefined),
  connectorKind: z.enum(ConnectorKind).optional().catch(undefined),
  connector: z.string().optional().catch(undefined),
  connectorSearch: z.string().optional().catch(undefined),
  settings: z.enum(SettingsPanel).optional().catch(undefined),
  teamView: z.enum(TeamSettingsView).optional().catch(undefined),
  inviteToken: z.string().optional().catch(undefined),
});

export const Route = createFileRoute("/_app")({
  validateSearch: searchParams,
  beforeLoad: async ({ context, location }) => {
    if (!context.authConfig.issuer) {
      return { session: DEFAULT_SESSION };
    }
    try {
      const response = await queryClient.ensureQueryData(
        createGetSessionQueryOptions({ transport }),
      );
      return { session: sessionFromResponse(response) };
    } catch (error) {
      if (ConnectError.from(error).code !== Code.Unauthenticated) {
        throw error;
      }
      throw redirect({ to: "/login", search: { returnTo: location.href } });
    }
  },
  component: AppLayout,
});
