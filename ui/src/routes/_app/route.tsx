import { createFileRoute, useLocation } from "@tanstack/react-router";
import { useAutoSignin } from "react-oidc-context";
import { z } from "zod";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import AppLayout from "@/layouts/app/AppLayout";
import { Flow } from "@/layouts/app/types";
import PendingLayout from "@/layouts/PendingLayout";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import type { SigninState } from "@/auth/types";

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

const AuthenticatedAppLayout = () => {
  const { href } = useLocation();
  const { isAuthenticated } = useAutoSignin({
    signinArgs: { state: { returnTo: href } satisfies SigninState },
  });

  return isAuthenticated ? <AppLayout /> : <PendingLayout />;
};

const AppRoute = () => {
  const { userManager } = Route.useRouteContext();

  return userManager ? <AuthenticatedAppLayout /> : <AppLayout />;
};

export const Route = createFileRoute("/_app")({
  validateSearch: searchParams,
  component: AppRoute,
});
