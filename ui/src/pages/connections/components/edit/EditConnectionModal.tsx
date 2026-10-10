import { type FC, useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type Connection,
  ConnectionSchema,
  GetConnectionRequestSchema,
  UpdateConnectionRequestSchema,
} from "@/gen/ingestion/v1/connections_pb";

import { ConnectionFormActionType } from "@/pages/connections/components/form/actions";
import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import { ConnectionFormPhase } from "@/pages/connections/components/form/types";

import { useFilamentLayoutSearch } from "@/module/hooks";

import { useGetConnectionQuery, useUpdateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface EditConnectionModalProps {
  onClose: () => void;
}

interface EditConnectionModalContentProps extends EditConnectionModalProps {
  connection: Connection;
}

const EditConnectionModalContent: FC<EditConnectionModalContentProps> = ({
  connection,
  onClose,
}) => {
  const { state, dispatch } = useConnectionFormContext();
  const { toast } = useToast();

  const { mutate: updateConnection } = useUpdateConnectionMutation();

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
          kind: connection.kind,
          connector: connection.connector,
          name,
          config: state.config,
          secretRefs: connection.secretRefs,
          version: connection.version,
        }),
      }),
      {
        onSuccess: () => {
          toast({
            variant: ToastVariant.SUCCESS,
            header: "Connection updated",
            description: `${name} has been updated successfully.`,
          });
          onClose();
        },
        onError: (error) => {
          dispatch({
            type: ConnectionFormActionType.SET_PHASE,
            payload: ConnectionFormPhase.ERROR,
          });
          const isConflict = ConnectError.from(error).code === Code.Aborted;
          toast({
            variant: ToastVariant.ERROR,
            header: isConflict ? "Connection changed elsewhere" : "Update failed",
            description: isConflict
              ? "This connection was modified since you opened it. Close the editor and reopen it to load the latest version."
              : getErrorMessage(error, "Update failed"),
          });
        },
      },
    );
  }, [state.name, state.config, connection, updateConnection, toast, onClose, dispatch]);

  return (
    <ConnectionForm
      connectorName={connection.connector}
      connectorKind={connection.kind}
      connectionId={connection.id}
      secretRefs={connection.secretRefs}
      onSubmit={handleUpdateConnection}
      onClose={onClose}
    />
  );
};

const EditConnectionModal: FC<EditConnectionModalProps> = ({ onClose }) => {
  const { connectionId } = useFilamentLayoutSearch();

  const { data, isError } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId ?? "" }),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = data?.connection;

  if (isError) {
    return (
      <ConnectionFormWrapper size={ModalSize.MEDIUM} header="Edit connection" onClose={onClose}>
        <ErrorLayout
          header="Connection not found"
          description="This connection no longer exists."
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      </ConnectionFormWrapper>
    );
  }

  if (!connection) {
    return (
      <ConnectionFormWrapper size={ModalSize.MEDIUM} header="Edit connection" onClose={onClose}>
        <PendingLayout />
      </ConnectionFormWrapper>
    );
  }

  return (
    <ConnectionFormProvider
      initialState={{ name: connection.name, config: connection.config ?? {} }}
    >
      <EditConnectionModalContent connection={connection} onClose={onClose} />
    </ConnectionFormProvider>
  );
};

export default EditConnectionModal;
