import type { FC } from "react";

import { KeyIcon } from "@phosphor-icons/react";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import WebhookEndpointErrorLayout from "@/components/webhooks/WebhookEndpointErrorLayout";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";
import ConnectionDrawerWebhookSigningKey from "@/pages/connections/components/drawer/ConnectionDrawerWebhookSigningKey";

import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

import { isNotFoundError } from "@/utils/errors";

interface ConnectionDrawerWebhookSecretsProps {
  connection: Connection;
}

const ConnectionDrawerWebhookSecrets: FC<ConnectionDrawerWebhookSecretsProps> = ({
  connection,
}) => {
  const { data, error, isError, isLoading } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(connection.id),
    options: { retry: false },
  });
  const endpoint = data?.endpoint;
  const isFailed = isError && !isNotFoundError(error);

  const renderContent = () => {
    if (isLoading) {
      return (
        <Box padding={12}>
          <Skeleton lines={2} />
        </Box>
      );
    }

    if (isFailed) {
      return <WebhookEndpointErrorLayout error={error} />;
    }

    return <ConnectionDrawerWebhookSigningKey connection={connection} endpoint={endpoint} />;
  };

  return (
    <ConnectionDrawerSection
      header="Secrets"
      icon={KeyIcon}
      count={endpoint?.hasSigningSecret ? 1 : undefined}
      emptyHeader="No secrets"
      emptyMessage="This connection has no secret references."
      isFlush
    >
      {renderContent()}
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerWebhookSecrets;
