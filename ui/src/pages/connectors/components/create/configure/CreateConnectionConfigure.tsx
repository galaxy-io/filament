import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate, useSearch } from "@tanstack/react-router";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import { ConnectionFormActionType } from "@/pages/connectors/components/form/actions";
import ConnectionForm from "@/pages/connectors/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connectors/components/form/ConnectionFormProvider";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";

import { useCreateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface CreateConnectionConfigureProps {
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureContent = ({ onClose, onBack }: CreateConnectionConfigureProps) => {
  const navigate = useNavigate();
  const { connector, connectorKind } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;
  const { state, dispatch } = useConnectionFormContext();
  const { showToast } = useToast();

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
          showToast({
            variant: ToastVariant.SUCCESS,
            header: "Connection created",
            subheader: `${name} has been created successfully.`,
          });

          if (response.connection?.id) {
            void navigate({
              to: ".",
              search: (prev) => ({
                ...prev,
                flow: undefined,
                connectionId: response.connection?.id,
              }),
            });
          }
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
          showToast({
            variant: ToastVariant.ERROR,
            header: "Creation failed",
            subheader: getErrorMessage(error, "Creation failed"),
          });
        },
      },
    );
  }, [state.name, state.config, connector, kind, createConnection, showToast, navigate, dispatch]);

  return (
    <ConnectionForm
      connectorName={connector ?? ""}
      connectorKind={kind}
      onSubmit={handleCreateConnection}
      onClose={onClose}
      onBack={onBack}
    />
  );
};

const CreateConnectionConfigure = ({ onClose, onBack }: CreateConnectionConfigureProps) => (
  <ConnectionFormProvider initialState={{ name: "", config: {} }}>
    <CreateConnectionConfigureContent onClose={onClose} onBack={onBack} />
  </ConnectionFormProvider>
);

export default CreateConnectionConfigure;
