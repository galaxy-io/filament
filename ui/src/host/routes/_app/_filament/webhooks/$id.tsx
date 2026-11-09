import { createFileRoute } from "@tanstack/react-router";

import WebhookGuidePage from "@/pages/connections/WebhookGuidePage";

import { webhookGuideRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/webhooks/$id")({
  ...webhookGuideRouteOptions,
  component: WebhookGuidePage,
});
