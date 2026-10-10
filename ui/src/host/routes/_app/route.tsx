import { Code, ConnectError } from "@connectrpc/connect";
import { createFileRoute, redirect } from "@tanstack/react-router";

import { settingsSearchSchema } from "@/module/schemas";

import { queryClient } from "@/host/api/queryClient";
import { transport } from "@/host/api/transport";
import AppLayout from "@/host/layouts/app/AppLayout";

import { createGetSessionQueryOptions } from "@/api/queries/auth";

import { DEFAULT_SESSION } from "@/auth/constants";
import { sessionFromResponse } from "@/auth/utils";

export const Route = createFileRoute("/_app")({
  validateSearch: settingsSearchSchema,
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
