import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/_app/_main/")({
  beforeLoad: () => {
    throw redirect({ to: "/observability" });
  },
});
