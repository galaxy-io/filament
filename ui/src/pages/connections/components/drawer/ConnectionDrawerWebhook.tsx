import { type FC, Fragment } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DocsLink from "@/components/DocsLink";
import { PIPELINE_CARD_HEIGHT } from "@/components/pipelines/constants";
import { WEBHOOK_DOCS_PATH } from "@/components/webhooks/constants";
import { WebhookEndpointStatus } from "@/components/webhooks/types";
import { getWebhookEndpointStatus } from "@/components/webhooks/utils";
import WebhookDeliveryUrlCopyButton from "@/components/webhooks/WebhookDeliveryUrlCopyButton";
import WebhookEndpointErrorLayout from "@/components/webhooks/WebhookEndpointErrorLayout";
import WebhookEndpointStatusBeacon from "@/components/webhooks/WebhookEndpointStatusBeacon";
import WebhookEndpointStatusCallout from "@/components/webhooks/WebhookEndpointStatusCallout";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

import { createGetPipelineInput, useGetPipelineQuery } from "@/api/queries/pipelines";
import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

import { isNotFoundError } from "@/utils/errors";

const NEEDS_PIPELINE_STATUSES = new Set([
  WebhookEndpointStatus.NO_PIPELINE,
  WebhookEndpointStatus.PIPELINE_DELETED,
]);

interface ConnectionDrawerWebhookProps {
  connection: Connection;
}

const ConnectionDrawerWebhook: FC<ConnectionDrawerWebhookProps> = ({ connection }) => {
  const openFlow = useFilamentFlowOpen();
  const { data, error, isError, isLoading } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(connection.id),
    options: { retry: false },
  });
  const endpoint = data?.endpoint;
  const isFailed = isError && !isNotFoundError(error);

  const { data: pipelineData, error: pipelineError } = useGetPipelineQuery({
    input: createGetPipelineInput(endpoint?.pipelineId ?? ""),
    options: { enabled: !!endpoint?.pipelineId, retry: false },
  });
  const isPipelineDeleted =
    isNotFoundError(pipelineError) || (pipelineData?.pipeline?.deletedAt ?? 0n) > 0n;
  const status = getWebhookEndpointStatus(endpoint, isPipelineDeleted);

  const renderContent = () => {
    if (isLoading) {
      return (
        <Box padding={12}>
          <Skeleton lines={3} />
        </Box>
      );
    }

    if (isFailed) {
      return <WebhookEndpointErrorLayout error={error} />;
    }

    const resources = endpoint?.resources ?? [];

    return (
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} fillWidth>
        <Box padding={12}>
          <WebhookEndpointStatusCallout
            status={status}
            actions={
              <Flex alignItems={AlignItems.CENTER} gap={12}>
                {NEEDS_PIPELINE_STATUSES.has(status) && (
                  <Button
                    label="Create pipeline"
                    size={ButtonSize.SMALL}
                    onClick={() => openFlow(Flow.CREATE_PIPELINE)}
                  />
                )}
                <DocsLink label="Setup guide" path={WEBHOOK_DOCS_PATH} size={TextSize.BODY_SM} />
              </Flex>
            }
          />
        </Box>
        {resources.map((resource) => (
          <Fragment key={resource}>
            <Divider />
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              gap={12}
              height={PIPELINE_CARD_HEIGHT}
              padding={[0, 12]}
              fillWidth
            >
              <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
                {resource}
              </Text>
              <WebhookDeliveryUrlCopyButton
                sourceConnectionId={connection.id}
                resource={resource}
              />
            </Flex>
          </Fragment>
        ))}
      </Flex>
    );
  };

  return (
    <ConnectionDrawerSection
      header="Endpoints"
      icon={WebhooksLogoIcon}
      count={undefined}
      emptyHeader="No endpoint"
      emptyMessage="This source has no webhook endpoint."
      actions={isLoading || isFailed ? undefined : <WebhookEndpointStatusBeacon status={status} />}
      isOpenInitial
      isFlush
    >
      {renderContent()}
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerWebhook;
