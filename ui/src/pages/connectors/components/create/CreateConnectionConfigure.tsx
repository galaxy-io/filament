import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import { ConnectionFormActionType } from "@/pages/connectors/components/form/actions";
import ConnectionForm from "@/pages/connectors/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connectors/components/form/ConnectionFormProvider";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";

import { useCreateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface CreateConnectionConfigureProps {
  connector: ConnectorSpec;
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureContent = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionConfigureProps) => {
  const navigate = useNavigate();
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
        kind: connector.kind,
        connector: connector.name,
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
  }, [state.name, state.config, connector, createConnection, showToast, navigate, dispatch]);

  return (
    <ConnectionForm
      connector={connector}
      title={`New ${connector.displayName || connector.name} connection`}
      submitLabel="Create"
      submittingLabel="Creating..."
      onSubmit={handleCreateConnection}
      onClose={onClose}
      onBack={onBack}
    />
  );
};

const CreateConnectionConfigure = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionConfigureProps) => (
  <ConnectionFormProvider initialState={{ name: "", config: {} }}>
    <CreateConnectionConfigureContent connector={connector} onClose={onClose} onBack={onBack} />
  </ConnectionFormProvider>
);

export default CreateConnectionConfigure;
