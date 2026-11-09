import type { FC } from "react";

import { ArrowLeftIcon, LinkBreakIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import CodeBlock, { CodeBlockLanguage } from "@galaxy-io/dls/text/CodeBlock";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Widget from "@galaxy-io/dls/widget/Widget";

import RouterLink from "@/components/RouterLink";

import { useFilamentNavigate, useWebhookParams } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

import { createGetConnectionInput, useSuspenseGetConnectionQuery } from "@/api/queries/connections";
import { createGetWebhookEndpointInput, useGetWebhookEndpointQuery } from "@/api/queries/webhooks";

import { isNotFoundError } from "@/utils/errors";
import { createWebhookCurlExample, webhookDeliveryTarget } from "@/utils/webhooks";

const WebhookGuidePage: FC = () => {
  const { id } = useWebhookParams();
  const navigate = useFilamentNavigate();
  const { data: connectionData } = useSuspenseGetConnectionQuery({
    input: createGetConnectionInput(id),
  });
  const { data, error, isError, isLoading, refetch } = useGetWebhookEndpointQuery({
    input: createGetWebhookEndpointInput(id),
    options: { retry: false },
  });
  const connection = connectionData.connection;
  const endpoint = data?.endpoint;
  const resources = [...(endpoint?.resources ?? [])].sort((a, b) => a.localeCompare(b));
  const exampleResource = resources.includes("issues") ? "issues" : (resources[0] ?? "issues");
  const openSource = () =>
    void navigate({ to: FilamentPath.SOURCES, search: { connectionId: id } });

  if (connection?.connector !== "webhook" || connection.deletedAt) {
    return (
      <ErrorLayout
        icon={LinkBreakIcon}
        header="Webhook source not found"
        description="This source does not exist or is no longer available."
        actions={
          <Button
            label="All sources"
            href={createFilamentHref(FilamentPath.SOURCES)}
            as={RouterLink}
          />
        }
      />
    );
  }

  const header = (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Button
        icon={ArrowLeftIcon}
        ariaLabel="All sources"
        tooltip="All sources"
        variant={ButtonVariant.TERTIARY}
        href={createFilamentHref(FilamentPath.SOURCES)}
        as={RouterLink}
      />
      <Text as="h1">Use {connection.name} webhook</Text>
    </Flex>
  );

  if (isLoading) {
    return (
      <PageLayout header={header}>
        <PendingLayout />
      </PageLayout>
    );
  }

  if (isError && !isNotFoundError(error)) {
    return (
      <PageLayout header={header}>
        <ErrorLayout
          header="Could not load webhook endpoint"
          description="The endpoint settings are unavailable."
          detail={error?.message}
          actions={<Button label="Retry" onClick={() => void refetch()} />}
        />
      </PageLayout>
    );
  }

  return (
    <PageLayout
      header={header}
      actions={
        <Button label="Open source" variant={ButtonVariant.SECONDARY} onClick={openSource} />
      }
    >
      <ScrollArea>
        <Flex
          alignItems={AlignItems.STRETCH}
          direction={FlexDirection.COLUMN}
          gap={12}
          padding={16}
          minWidth={400}
          maxWidth={760}
        >
          {!endpoint && (
            <Alert variant={AlertVariant.WARNING} header="Endpoint not configured">
              Create a continuous pipeline, then open this source to select its destination and save
              a signing key.
            </Alert>
          )}
          {endpoint && !endpoint.enabled && (
            <Alert variant={AlertVariant.WARNING} header="Deliveries are off">
              Open this source and turn on Accept deliveries after its pipeline is ready.
            </Alert>
          )}
          {endpoint?.enabled && !endpoint.accepting && (
            <Alert variant={AlertVariant.ERROR} header="No pipeline is reading this source">
              New deliveries get 503 and senders retry them later. Start the continuous pipeline to
              accept them again.
            </Alert>
          )}
          {endpoint?.enabled && endpoint.accepting && !endpoint.routeReady && (
            <Alert variant={AlertVariant.WARNING} header="Deliveries are waiting">
              Deliveries are stored until the selected continuous pipeline runs and reads these
              resources. Deliveries nothing reads are dropped after the hold period.
            </Alert>
          )}

          <Widget header="Webhook URLs" gap={12}>
            {!endpoint?.deliveryUrl && (
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Send POST requests to your public Filament origin followed by one of these paths.
                Set WEBHOOK_PUBLIC_BASE_URL on the server to show full URLs here.
              </Text>
            )}
            {resources.length === 0 && (
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                URLs appear here after the endpoint is saved.
              </Text>
            )}
            {resources.map((resource) => (
              <CopyInput
                key={resource}
                label={`${resource} webhook ${endpoint?.deliveryUrl ? "URL" : "path"}`}
                value={webhookDeliveryTarget(id, resource, endpoint?.deliveryUrl)}
                family={FontFamily.MONO}
                fillWidth
              />
            ))}
          </Widget>

          <Widget header="Sign each delivery" gap={12}>
            <Text size={TextSize.BODY_SM}>
              Choose Generate a key and save the endpoint in the source drawer. Filament shows that
              key once. Store it in your sender. If you already have a key, choose Import sender key
              instead.
            </Text>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Your sender generates a fresh HMAC-SHA256 signature for every request. Sign the exact
              UTF-8 bytes of timestamp, a dot, delivery ID, a dot, then the unmodified request body.
              Send the lowercase hex digest with a sha256= prefix.
            </Text>
            <CodeBlock
              content="<timestamp>.<delivery-id>.<raw-body>"
              label="Signed bytes"
              canCopy
            />
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Send X-Filament-Timestamp as Unix seconds, X-Filament-Delivery-Id as a unique event
              ID, and X-Filament-Signature as the digest. Keep the timestamp within five minutes of
              the server clock.
            </Text>
          </Widget>

          <Widget header="Try a delivery" gap={12}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Run this command first and enter the signing key shown when you saved the endpoint.
            </Text>
            <CodeBlock
              content={"printf 'Signing key: '; read -r -s FILAMENT_WEBHOOK_KEY; printf '\\n'"}
              language={CodeBlockLanguage.BASH}
              canCopy
            />
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Then copy the request below into the same Bash terminal. It signs one JSON body and
              sends those exact bytes with curl.
            </Text>
            <CodeBlock
              content={createWebhookCurlExample(id, exampleResource, endpoint?.deliveryUrl)}
              language={CodeBlockLanguage.BASH}
              label={`send-${exampleResource}.sh`}
              canCopy
            />
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              The default accepted response is HTTP 202. A retry with the same delivery ID and
              identical body returns the original receipt while retained.
            </Text>
          </Widget>
        </Flex>
      </ScrollArea>
    </PageLayout>
  );
};

export default WebhookGuidePage;
