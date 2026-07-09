import { createFileRoute } from "@tanstack/react-router";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

import ProvidersPage from "@/pages/providers/ProvidersPage";

export const Route = createFileRoute("/sinks")({
  component: () => <ProvidersPage kind={ProviderKind.SINK} />,
});
