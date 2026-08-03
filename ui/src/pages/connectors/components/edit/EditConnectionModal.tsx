import { useCallback, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type Connection,
  ConnectionSchema,
  UpdateConnectionRequestSchema,
} from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import { getConnectionScopedFields } from "@/components/fields/utils";

import { ConnectionFormActionType } from "@/pages/connectors/components/form/actions";
import ConnectionForm from "@/pages/connectors/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connectors/components/form/ConnectionFormProvider";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";
import { omitBlankSecretFields } from "@/pages/connectors/components/form/utils";
import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";

import { useUpdateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface EditConnectionModalProps {
  connection: Connection;
  onClose: () => void;
}

interface EditConnectionModalContentProps extends EditConnectionModalProps {
  connector: ConnectorSpec;
}

const EditConnectionModalContent = ({
  connection,
  connector,
  onClose,
}: EditConnectionModalContentProps) => {
  const { state, dispatch } = useConnectionFormContext();
  const { showToast } = useToast();

  const { mutate: updateConnection } = useUpdateConnectionMutation();

  const fields = useMemo(
    () => getConnectionScopedFields(connector.configSchema?.fields ?? []),
    [connector],
  );

  const handleUpdateConnection = useCallback(() => {
    const name = state.name.trim();
    if (!name) return;

    dispatch({
      type: ConnectionFormActionType.SET_PHASE,
      payload: ConnectionFormPhase.SUBMITTING,
    });

    updateConnection(
      create(UpdateConnectionRequestSchema, {
        connection: create(ConnectionSchema, {
          id: connection.id,
          tenantId: connection.tenantId,
          kind: connection.kind,
          connector: connection.connector,
          name,
          config: omitBlankSecretFields(fields, state.config),
          secretRefs: connection.secretRefs,
          version: connection.version,
        }),
      }),
      {
        onSuccess: () => {
          showToast({
            variant: ToastVariant.SUCCESS,
            header: "Connection updated",
            subheader: `${name} has been updated successfully.`,
          });
          onClose();
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
          const isConflict = ConnectError.from(error).code === Code.Aborted;
          showToast({
            variant: ToastVariant.ERROR,
            header: isConflict ? "Connection changed elsewhere" : "Update failed",
            subheader: isConflict
              ? "This connection was modified since you opened it. Close the editor and reopen it to load the latest version."
              : getErrorMessage(error, "Update failed"),
          });
        },
      },
    );
  }, [
    state.name,
    state.config,
    fields,
    connection,
    updateConnection,
    showToast,
    onClose,
    dispatch,
  ]);

  return (
    <ConnectionForm
      connector={connector}
      connectionId={connection.id}
      title={`Edit ${connector.displayName || connector.name} connection`}
      submitLabel="Save"
      submittingLabel="Saving..."
      onSubmit={handleUpdateConnection}
      onClose={onClose}
    />
  );
};

const EditConnectionModal = ({ connection, onClose }: EditConnectionModalProps) => {
  const connector = useConnectorSpec(connection.connector, connection.kind);

  if (!connector) return null;

  return (
    <ConnectionFormProvider
      initialState={{ name: connection.name, config: connection.config ?? {} }}
    >
      <EditConnectionModalContent connection={connection} connector={connector} onClose={onClose} />
    </ConnectionFormProvider>
  );
};

export default EditConnectionModal;
