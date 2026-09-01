import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import AppLayout from "@/layouts/app/AppLayout";
import { Flow } from "@/layouts/app/types";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { ensureSession, redirectToSignIn } from "@/auth/oidc";

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
  beforeLoad: async ({ context, location, preload }) => {
    if (!context.authConfig.issuer) {
      return;
    }
    if (await ensureSession()) {
      return;
    }
    if (preload) {
      throw new Error("Unauthenticated");
    }
    await redirectToSignIn(location.href);
  },
  component: AppLayout,
});
