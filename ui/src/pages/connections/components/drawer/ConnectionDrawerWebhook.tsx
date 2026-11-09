import { type FC, useState } from "react";

import { LinkIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionDrawerWebhookForm from "@/pages/connections/components/drawer/ConnectionDrawerWebhookForm";
import { connectionDrawerWebhookRoutes } from "@/pages/connections/components/drawer/webhook/utils";
import { useConnectionPipelines } from "@/pages/connections/hooks/useConnectionPipelines";

import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

import { isNotFoundError } from "@/utils/errors";

interface ConnectionDrawerWebhookProps {
  connection: Connection;
}

const ConnectionDrawerWebhook: FC<ConnectionDrawerWebhookProps> = ({ connection }) => {
  const [mintedSecret, setMintedSecret] = useState("");
  const { data, error, isError, isLoading, refetch } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(connection.id),
    options: { retry: false },
  });
  const {
    pipelines,
    isError: isPipelinesError,
    isLoading: isPipelinesLoading,
    refetch: refetchPipelines,
  } = useConnectionPipelines();
  const endpoint = data?.endpoint;
  const routes = connectionDrawerWebhookRoutes(pipelines, connection.id);

  const handleRefresh = () => {
    void refetch();
    void refetchPipelines();
  };

  const renderContent = () => {
    if (isLoading) {
      return <Skeleton lines={4} />;
    }
    if (isError && !isNotFoundError(error)) {
      return (
        <Flex direction={FlexDirection.COLUMN} gap={12}>
          <Alert variant={AlertVariant.ERROR} header="Endpoint unavailable">
            {error?.message}
          </Alert>
          <Button label="Retry" variant={ButtonVariant.SECONDARY} onClick={handleRefresh} />
        </Flex>
      );
    }
    return (
      <ConnectionDrawerWebhookForm
        key={endpoint?.revision.toString() ?? "new"}
        connection={connection}
        endpoint={endpoint}
        routes={routes}
        isPipelinesLoading={isPipelinesLoading}
        isPipelinesError={isPipelinesError}
        mintedSecret={mintedSecret}
        onMintedSecret={setMintedSecret}
        onRefresh={handleRefresh}
      />
    );
  };

  return (
    <Widget isCollapsible defaultIsOpen header="Webhook endpoint" icon={LinkIcon}>
      {renderContent()}
    </Widget>
  );
};

export default ConnectionDrawerWebhook;
