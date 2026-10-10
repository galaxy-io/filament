import { type FC, useCallback } from "react";

import { create } from "@bufbuild/protobuf";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import { ConnectionFormActionType } from "@/pages/connections/components/form/actions";
import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import { ConnectionFormPhase } from "@/pages/connections/components/form/types";

import { useFilamentLayoutSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { useCreateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface CreateConnectionConfigureProps {
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureContent: FC<CreateConnectionConfigureProps> = ({
  onClose,
  onBack,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const { connector, connectorKind } = useFilamentLayoutSearch();
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;
  const { state, dispatch } = useConnectionFormContext();
  const { toast } = useToast();

  const { mutate: createConnection } = useCreateConnectionMutation();

  const handleCreateConnection = useCallback(() => {
    const name = state.name.trim();
    if (!name) return;

    dispatch({
      type: ConnectionFormActionType.SET_PHASE,
      payload: ConnectionFormPhase.SUBMITTING,
    });

    createConnection(
      create(CreateConnectionRequestSchema, {
        kind,
        connector: connector ?? "",
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
              connectionId: response.connection?.id,
            }));
          }
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
          toast({
            variant: ToastVariant.ERROR,
            header: "Creation failed",
            description: getErrorMessage(error, "Creation failed"),
          });
        },
      },
    );
  }, [state.name, state.config, connector, kind, createConnection, toast, updateSearch, dispatch]);

  const handleConnectionChange = useCallback(
    (version: string) => {
      void updateSearch((prev) => ({ ...prev, connector: version }));
    },
    [updateSearch],
  );

  return (
    <ConnectionForm
      connectorName={connector ?? ""}
      connectorKind={kind}
      onConnectorChange={handleConnectionChange}
      onSubmit={handleCreateConnection}
      onClose={onClose}
      onBack={onBack}
    />
  );
};

const CreateConnectionConfigure: FC<CreateConnectionConfigureProps> = ({ onClose, onBack }) => (
  <ConnectionFormProvider initialState={{ name: "", config: {} }}>
    <CreateConnectionConfigureContent onClose={onClose} onBack={onBack} />
  </ConnectionFormProvider>
);

export default CreateConnectionConfigure;
