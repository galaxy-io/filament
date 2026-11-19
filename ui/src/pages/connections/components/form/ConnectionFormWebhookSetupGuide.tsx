import type { FC } from "react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import MarkdownText from "@galaxy-io/dls/text/MarkdownText";

import type { WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";

import { createWebhookSetupMarkdown } from "@/components/webhooks/utils";

interface WebhookSetupGuideProps {
  webhook: WebhookSpec;
  deliveryUrl: string;
}

const ConnectionFormWebhookSetupGuide: FC<WebhookSetupGuideProps> = ({ webhook, deliveryUrl }) =>
  webhook.setup ? (
    <Box variant={BoxVariant.SECONDARY} padding={12}>
      <MarkdownText content={createWebhookSetupMarkdown(webhook.setup, deliveryUrl)} />
    </Box>
  ) : null;

export default ConnectionFormWebhookSetupGuide;
