import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/_app/pipelines/$id/")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/pipelines/$id/canvas",
      params: { id: params.id },
    });
  },
});
