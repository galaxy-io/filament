import { createFileRoute, redirect } from "@tanstack/react-router";

import { ensureSession, redirectToSignOut } from "@/auth/oidc";

export const Route = createFileRoute("/logout")({
  pendingMs: 0,
  pendingMinMs: 0,
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  loader: async () => {
    if (!(await ensureSession())) {
      throw redirect({ to: "/" });
    }
    await redirectToSignOut();
  },
});
