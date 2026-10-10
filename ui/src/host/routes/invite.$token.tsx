import { createFileRoute, redirect } from "@tanstack/react-router";

import InvitePage from "@/host/pages/auth/InvitePage";

export const Route = createFileRoute("/invite/$token")({
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  component: InvitePage,
});
