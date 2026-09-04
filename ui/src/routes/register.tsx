import { createFileRoute, redirect } from "@tanstack/react-router";

import RegisterPage from "@/pages/auth/RegisterPage";

export const Route = createFileRoute("/register")({
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  component: RegisterPage,
});
