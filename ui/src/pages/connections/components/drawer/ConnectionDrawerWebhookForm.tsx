import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import RadioGroup from "@galaxy-io/dls/inputs/RadioGroup";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  PutWebhookEndpointRequestSchema,
  type WebhookEndpointConfig,
  WebhookEndpointConfigSchema,
} from "@/gen/ingestion/v1/webhooks_pb";

import RouterLink from "@/components/RouterLink";

import {
  type ConnectionDrawerWebhookRoute,
  createConnectionDrawerWebhookState,
} from "@/pages/connections/components/drawer/webhook/utils";

import { useFilamentFlowOpen } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";
import { Flow } from "@/module/types";

import { usePutWebhookEndpointMutation } from "@/api/queries/webhooks";

import { getErrorMessage } from "@/utils/errors";
import { webhookDeliveryTarget } from "@/utils/webhooks";

interface ConnectionDrawerWebhookFormProps {
  connection: Connection;
  endpoint?: WebhookEndpointConfig;
  routes: ConnectionDrawerWebhookRoute[];
  isPipelinesLoading: boolean;
  isPipelinesError: boolean;
  mintedSecret: string;
  onMintedSecret: (secret: string) => void;
  onRefresh: () => void;
}

const ConnectionDrawerWebhookForm: FC<ConnectionDrawerWebhookFormProps> = ({
  connection,
  endpoint,
  routes,
  isPipelinesLoading,
  isPipelinesError,
  mintedSecret,
  onMintedSecret,
  onRefresh,
}) => {
  const { toast } = useToast();
  const openFlow = useFilamentFlowOpen();
  const [state, setState] = useState(() => ({
    ...createConnectionDrawerWebhookState(endpoint),
    error: "",
  }));
  const { mutate: putEndpoint, isPending } = usePutWebhookEndpointMutation();
  const savedRouteId =
    endpoint?.pipelineId && endpoint.routeKey ? `${endpoint.pipelineId}|${endpoint.routeKey}` : "";
  const routeOptions = routes.map((route) => ({ id: route.id, label: route.label }));
  if (savedRouteId && !routes.some((route) => route.id === savedRouteId)) {
    routeOptions.push({ id: savedRouteId, label: "Saved route" });
  }
  const selectedRoute =
    routes.find((route) => route.id === state.routeId) ??
    (state.routeId === savedRouteId && endpoint
      ? {
          pipelineId: endpoint.pipelineId,
          routeKey: endpoint.routeKey,
          resources: endpoint.resources,
        }
      : undefined);
  const importedSecretLength = new TextEncoder().encode(state.importedSecret).length;
  const secretError =
    state.secretChoice === "import" && (importedSecretLength < 16 || importedSecretLength > 4096)
      ? "Signing key must be 16–4096 bytes."
      : undefined;
  const currentResources = [...(selectedRoute?.resources ?? [])].sort();
  const isResourcesChanged =
    currentResources.join("\n") !== [...(endpoint?.resources ?? [])].sort().join("\n");
  const isDirty =
    !endpoint ||
    state.enabled !== endpoint.enabled ||
    state.routeId !== savedRouteId ||
    isResourcesChanged ||
    state.secretChoice !== "keep";
  const canSave = isDirty && !secretError && !!selectedRoute && currentResources.length > 0;

  const handleSave = () => {
    if (!canSave) return;
    setState((prev) => ({ ...prev, error: "" }));
    putEndpoint(
      create(PutWebhookEndpointRequestSchema, {
        endpoint: create(WebhookEndpointConfigSchema, {
          sourceConnectionId: connection.id,
          pipelineId: selectedRoute?.pipelineId ?? "",
          routeKey: selectedRoute?.routeKey ?? "",
          resources: currentResources,
          enabled: state.enabled,
          successStatus: endpoint?.successStatus ?? 202,
          maxBodyBytes: endpoint?.maxBodyBytes ?? 1048576n,
        }),
        expectedRevision: endpoint?.revision ?? 0n,
        importedSecret: state.secretChoice === "import" ? state.importedSecret : "",
        mintSecret: state.secretChoice === "mint",
      }),
      {
        onSuccess: (response) => {
          onMintedSecret(response.mintedSecret);
          toast({ header: "Webhook endpoint saved", variant: ToastVariant.SUCCESS });
        },
        onError: (error) => {
          const message = getErrorMessage(error, "Could not save webhook endpoint");
          setState((prev) => ({ ...prev, error: message }));
          toast({ header: "Save failed", description: message, variant: ToastVariant.ERROR });
        },
      },
    );
  };

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16} fillWidth>
      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
        Add resources to a continuous pipeline, then connect its route and signing key here.
      </Text>

      {mintedSecret && (
        <Flex direction={FlexDirection.COLUMN} gap={8}>
          <Alert variant={AlertVariant.WARNING} header="Copy the signing key now">
            This key is shown once. Add it to your sender before leaving this drawer.
          </Alert>
          <CopyInput label="Signing key" value={mintedSecret} family={FontFamily.MONO} fillWidth />
        </Flex>
      )}

      {endpoint?.enabled && !endpoint.accepting && (
        <Alert variant={AlertVariant.ERROR} header="No pipeline is reading this source">
          New deliveries get 503 and senders retry them later. Start the continuous pipeline to
          accept them again.
        </Alert>
      )}
      {endpoint?.enabled && endpoint.accepting && !endpoint.routeReady && (
        <Alert variant={AlertVariant.WARNING} header="Deliveries are waiting">
          Deliveries are stored until the continuous pipeline runs and reads every resource.
          Deliveries nothing reads are dropped after the hold period.
        </Alert>
      )}
      {endpoint && !endpoint.enabled && endpoint.pipelineId && !endpoint.routeReady && (
        <Alert variant={AlertVariant.WARNING} header="Pipeline not ready">
          Start the continuous pipeline, then refresh this endpoint before enabling acceptance.
        </Alert>
      )}

      <RadioGroup
        label="Signing key"
        options={
          endpoint?.hasSigningSecret
            ? [
                { id: "keep", label: "Keep current key" },
                { id: "mint", label: "Generate new key" },
                { id: "import", label: "Import sender key" },
              ]
            : [
                { id: "mint", label: "Generate a key" },
                { id: "import", label: "Import sender key" },
              ]
        }
        value={state.secretChoice}
        onChange={(choice) => {
          if (choice === "keep" || choice === "mint" || choice === "import") {
            setState((prev) => ({ ...prev, secretChoice: choice, error: "" }));
          }
        }}
      />
      {state.secretChoice === "import" && (
        <PasswordInput
          label="Sender signing key"
          value={state.importedSecret}
          onChange={(value) => setState((prev) => ({ ...prev, importedSecret: value }))}
          autoComplete="new-password"
          error={secretError}
          fillWidth
        />
      )}

      <Flex direction={FlexDirection.COLUMN} gap={8}>
        <SelectInput
          label="Pipeline destination"
          options={routeOptions}
          value={state.routeId || null}
          onChange={(routeId) =>
            setState((prev) => ({ ...prev, routeId: routeId ?? "", error: "" }))
          }
          placeholder="Select a continuous pipeline..."
          isLoading={isPipelinesLoading}
          isClearable
          fillWidth
        />
        {isPipelinesError && (
          <Alert variant={AlertVariant.ERROR}>Could not load continuous pipelines.</Alert>
        )}
        {!isPipelinesLoading && !isPipelinesError && routes.length === 0 && (
          <Flex direction={FlexDirection.COLUMN} gap={8}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Create a continuous pipeline using this source and add its webhook resources there.
            </Text>
            <Button
              label="Create pipeline"
              variant={ButtonVariant.SECONDARY}
              onClick={() => openFlow(Flow.CREATE_PIPELINE)}
            />
          </Flex>
        )}
      </Flex>

      {currentResources.map((resource) => (
        <Flex key={resource} direction={FlexDirection.COLUMN} gap={4}>
          <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
            {resource}
          </Text>
          {endpoint?.resources.includes(resource) && (
            <CopyInput
              label={endpoint.deliveryUrl ? "Webhook URL" : "Webhook path"}
              value={webhookDeliveryTarget(connection.id, resource, endpoint.deliveryUrl)}
              family={FontFamily.MONO}
              fillWidth
            />
          )}
        </Flex>
      ))}

      <CheckboxInput
        label="Accept deliveries"
        description="Turning this on requires the selected pipeline to be running. Deliveries are then stored until the pipeline reads them. While every pipeline reading this source is stopped, senders get 503 and retry."
        isChecked={state.enabled}
        onChange={(enabled) => setState((prev) => ({ ...prev, enabled, error: "" }))}
      />

      {state.error && (
        <Alert variant={AlertVariant.ERROR} header="Save failed">
          {state.error}
        </Alert>
      )}

      <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
        <Button
          label="Usage guide"
          variant={ButtonVariant.SECONDARY}
          href={createFilamentHref(FilamentPath.WEBHOOK_GUIDE, { id: connection.id })}
          as={RouterLink}
        />
        <Button label="Refresh" variant={ButtonVariant.SECONDARY} onClick={onRefresh} />
        <Button
          label="Save endpoint"
          isDisabled={!canSave}
          isLoading={isPending}
          onClick={handleSave}
          hotKeys={["mod", "enter"]}
          shouldBindHotKey
        />
      </Flex>
    </Flex>
  );
};

export default ConnectionDrawerWebhookForm;
