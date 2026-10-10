import { type FC, useCallback } from "react";

import { create } from "@bufbuild/protobuf";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormActions,
  useConnectionFormState,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import { ConnectionFormPhase } from "@/pages/connections/components/form/types";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { useCreateConnectionMutation } from "@/api/queries/connections";

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

  const { mutate: createConnection } = useCreateConnectionMutation();

  const handleCreateConnection = useCallback(() => {
    const name = state.name.trim();
    if (!name) return;

    setPhase(ConnectionFormPhase.SUBMITTING);

    createConnection(
      create(CreateConnectionRequestSchema, {
        kind,
        connector,
        name,
        config: state.config,
      }),
      {
        onSuccess: (response) => {
          toast({
            variant: ToastVariant.SUCCESS,
            header: "Connection created",
            description: `${name} has been created successfully.`,
          });

          if (response.connection?.id) {
            void updateSearch((prev) => ({
              ...prev,
              flow: undefined,
              connector: undefined,
              connectorKind: undefined,
              connectorSearch: undefined,
              connectionId: response.connection?.id,
            }));
          }
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
  }, [state.name, state.config, connector, kind, createConnection, toast, updateSearch, setPhase]);

  const handleConnectionChange = useCallback(
    (version: string) => {
      void updateSearch((prev) => ({ ...prev, connector: version }));
    },
    [updateSearch],
  );

  return (
    <ConnectionForm
      isOpen={isOpen}
      connectorName={connector}
      connectorKind={kind}
      onConnectorChange={handleConnectionChange}
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
