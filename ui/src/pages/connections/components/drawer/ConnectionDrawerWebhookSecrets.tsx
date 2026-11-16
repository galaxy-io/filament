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
  hasConnectionSecrets: boolean;
  isProviderSecret: boolean;
}

const ConnectionDrawerWebhookSecrets: FC<ConnectionDrawerWebhookSecretsProps> = ({
  connection,
  hasConnectionSecrets,
  isProviderSecret,
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

    return (
      <ConnectionDrawerWebhookSigningKey
        connection={connection}
        endpoint={endpoint}
        isProviderSecret={isProviderSecret}
      />
    );
  };

  return (
    <ConnectionDrawerSection
      header={hasConnectionSecrets ? "Webhook signing key" : "Secrets"}
      icon={KeyIcon}
      count={endpoint?.hasSigningSecret ? 1 : undefined}
      emptyHeader={hasConnectionSecrets ? "No signing key" : "No secrets"}
      emptyMessage={
        hasConnectionSecrets
          ? "This connection has no webhook signing key."
          : "This connection has no secret references."
      }
      isFlush
    >
      {renderContent()}
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerWebhookSecrets;
