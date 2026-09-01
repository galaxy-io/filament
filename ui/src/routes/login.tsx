import { createClient } from "@connectrpc/connect";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { z } from "zod";

import { AuthService } from "@/gen/auth/v1/service_pb";

import LoginPage from "@/pages/auth/LoginPage";

import { transport } from "@/api/transport";

const searchParams = z.object({
  authRequest: z.string().optional().catch(undefined),
});

const authClient = createClient(AuthService, transport);

const resumeLogin = async (authRequestId: string): Promise<string | undefined> => {
  try {
    const { callbackUrl } = await authClient.resumeLogin({ authRequestId });
    return callbackUrl;
  } catch {
    return undefined;
  }
};

export const Route = createFileRoute("/login")({
  validateSearch: searchParams,
  loaderDeps: ({ search: { authRequest } }) => ({ authRequest }),
  pendingMs: 0,
  pendingMinMs: 0,
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  loader: async ({ deps: { authRequest } }) => {
    if (!authRequest) {
      return;
    }
    const callbackUrl = await resumeLogin(authRequest);
    if (callbackUrl) {
      throw redirect({ href: callbackUrl });
    }
  },
  component: LoginPage,
});
