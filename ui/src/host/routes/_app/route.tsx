import { Code, ConnectError } from "@connectrpc/connect";
import { createFileRoute, redirect } from "@tanstack/react-router";

import { queryClient } from "@/host/api/queryClient";
import { transport } from "@/host/api/transport";
import { DEFAULT_SESSION } from "@/host/auth/constants";
import { mapSessionResponseToAppSession } from "@/host/auth/utils";
import AppLayout from "@/host/layouts/app/AppLayout";

import { createGetSessionQueryOptions } from "@/api/queries/auth";

export const Route = createFileRoute("/_app")({
  beforeLoad: async ({ context, location }) => {
    if (!context.authConfig.issuer) {
      return { session: DEFAULT_SESSION };
    }
    try {
      const response = await queryClient.ensureQueryData(
        createGetSessionQueryOptions({ transport }),
      );
      return { session: mapSessionResponseToAppSession(response) };
    } catch (error) {
      if (ConnectError.from(error).code !== Code.Unauthenticated) {
        throw error;
      }
      throw redirect({ to: "/login", search: { returnTo: location.href } });
    }
  },
  component: AppLayout,
});
