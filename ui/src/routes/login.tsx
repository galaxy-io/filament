import { createFileRoute, redirect } from "@tanstack/react-router";
import { z } from "zod";

import LoginPage from "@/pages/auth/LoginPage";

const searchParams = z.object({
  returnTo: z.string().optional().catch(undefined),
});

export const Route = createFileRoute("/login")({
  validateSearch: searchParams,
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  component: LoginPage,
});
