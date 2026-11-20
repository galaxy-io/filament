import type { FC } from "react";

import type { ConnectError } from "@connectrpc/connect";

import Box from "@galaxy-io/dls/layout/Box";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";

import { isUnimplementedError } from "@/utils/errors";

interface WebhookEndpointErrorLayoutProps {
  error: ConnectError | null;
}

const WebhookEndpointErrorLayout: FC<WebhookEndpointErrorLayoutProps> = ({ error }) => {
  if (isUnimplementedError(error)) {
    return (
      <Box padding={12}>
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header="Webhooks unavailable"
          description="Webhook endpoints need PostgreSQL. This server does not support them."
        />
      </Box>
    );
  }

  return (
    <Box padding={12}>
      <ErrorLayout
        size={ErrorLayoutSize.SMALL}
        header="Failed to load endpoint"
        description="The webhook endpoint for this source could not be loaded."
        detail={error?.message}
      />
    </Box>
  );
};

export default WebhookEndpointErrorLayout;
