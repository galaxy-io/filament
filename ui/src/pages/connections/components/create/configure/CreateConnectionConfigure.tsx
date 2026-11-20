import { type FC, useCallback, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { match } from "ts-pattern";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type Connection, CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec, WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";

import { isWebhookProviderIssued, isWebhookReceiving } from "@/components/webhooks/utils";

import { createConnectionCreatedSearch } from "@/pages/connections/components/create/utils";
import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormActions,
  useConnectionFormState,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import ConnectionFormSigningKeyModal from "@/pages/connections/components/form/ConnectionFormSigningKeyModal";
import ConnectionFormWebhookSecretModal from "@/pages/connections/components/form/ConnectionFormWebhookSecretModal";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
  type ConnectionFormWebhookSetup,
  ConnectionFormWebhookStep,
} from "@/pages/connections/components/form/types";
import { useConnectionWebhookKeyMint } from "@/pages/connections/hooks/useConnectionWebhookKeyMint";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { useCreateConnectionMutation } from "@/api/queries/connections";
import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

import { getErrorMessage } from "@/utils/errors";

interface CreateConnectionConfigureProps {
  isOpen: boolean;
  connector: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureContent: FC<CreateConnectionConfigureProps> = ({
  isOpen,
  connector,
  connectorKind: kind,
  onClose,
  onBack,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const state = useConnectionFormState();
  const { setPhase } = useConnectionFormActions();
  const { toast } = useToast();

  const [webhookSetup, setWebhookSetup] = useState<ConnectionFormWebhookSetup | undefined>();

  const { data: connectorData } = useGetConnectorQuery({
    input: createGetConnectorInput(connector, kind),
  });

  const { mutate: createConnection } = useCreateConnectionMutation();
  const { mintKey } = useConnectionWebhookKeyMint();

  const completeCreate = useCallback(
    (connection: Connection) => {
      void updateSearch((prev) => createConnectionCreatedSearch(prev, connection));
    },
    [updateSearch],
  );

  const mintSigningKey = useCallback(
    (connection: Connection, webhook: WebhookSpec) => {
      mintKey(connection, webhook, undefined, {
        onSuccess: (reveal) => setWebhookSetup({ step: ConnectionFormWebhookStep.REVEAL, reveal }),
        onError: (error) => {
          toast({
            variant: ToastVariant.ERROR,
            header: "Connection created without a signing key",
            description: getErrorMessage(error, "Could not generate a signing key"),
          });
          completeCreate(connection);
        },
      });
    },
    [mintKey, toast, completeCreate],
  );

  const handleCreateConnection = useCallback(
    (config: ConnectionFormState["config"]) => {
      const name = state.name.trim();
      if (!name) return;

      setPhase(ConnectionFormPhase.SUBMITTING);

      createConnection(
        create(CreateConnectionRequestSchema, {
          kind,
          connector,
          name,
          config,
        }),
        {
          onSuccess: (response) => {
            const connection = response.connection;
            if (!connection?.id) return;

            toast({
              variant: ToastVariant.SUCCESS,
              header: "Connection created",
            });

            const webhook = connectorData?.connector?.webhook;
            if (isWebhookReceiving(webhook, connection.config)) {
              if (isWebhookProviderIssued(webhook)) {
                setWebhookSetup({ step: ConnectionFormWebhookStep.SECRET, connection, webhook });
              } else {
                mintSigningKey(connection, webhook);
              }
              return;
            }
            completeCreate(connection);
          },
          onError: (error) => {
            setPhase(ConnectionFormPhase.ERROR);
            toast({
              variant: ToastVariant.ERROR,
              header: "Creation failed",
              description: getErrorMessage(error, "Creation failed"),
            });
          },
        },
      );
    },
    [
      state.name,
      connector,
      kind,
      createConnection,
      toast,
      setPhase,
      mintSigningKey,
      completeCreate,
      connectorData,
    ],
  );

  const handleConnectorVersionChange = useCallback(
    (version: ConnectorSpec["name"]) => {
      void updateSearch((prev) => ({ ...prev, connector: version }), { replace: true });
    },
    [updateSearch],
  );

  if (webhookSetup) {
    return match(webhookSetup)
      .with({ step: ConnectionFormWebhookStep.REVEAL }, ({ reveal }) => (
        <ConnectionFormSigningKeyModal
          isOpen={isOpen}
          reveal={reveal}
          onDone={() => completeCreate(reveal.connection)}
        />
      ))
      .with({ step: ConnectionFormWebhookStep.SECRET }, ({ connection, webhook }) => (
        <ConnectionFormWebhookSecretModal
          isOpen={isOpen}
          connection={connection}
          webhook={webhook}
          endpoint={undefined}
          isNewConnection
          onDone={() => completeCreate(connection)}
        />
      ))
      .exhaustive();
  }

  return (
    <ConnectionForm
      isOpen={isOpen}
      connectorName={connector}
      connectorKind={kind}
      onConnectorChange={handleConnectorVersionChange}
      onSubmit={handleCreateConnection}
      onClose={onClose}
      onBack={onBack}
    />
  );
};

const CreateConnectionConfigure: FC<CreateConnectionConfigureProps> = (props) => (
  <ConnectionFormProvider initialState={{ name: "", config: {} }}>
    <CreateConnectionConfigureContent {...props} />
  </ConnectionFormProvider>
);

export default CreateConnectionConfigure;
