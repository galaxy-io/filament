import { type FC, useCallback, useState } from "react";

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
  UpdateConnectionRequestSchema,
} from "@/gen/ingestion/v1/connections_pb";

import ConnectionForm from "@/pages/connections/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormActions,
  useConnectionFormState,
} from "@/pages/connections/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connections/components/form/types";

import type { FilamentLayoutSearch } from "@/module/schemas";

import {
  createGetConnectionInput,
  useGetConnectionQuery,
  useUpdateConnectionMutation,
} from "@/api/queries/connections";

import { useOverlaySession } from "@/hooks/useOverlaySession";

import { IS_DEBUG } from "@/constants";

import { getErrorMessage, isNotFoundError } from "@/utils/errors";

interface EditConnectionModalProps {
  isOpen: boolean;
  connectionId: FilamentLayoutSearch["connectionId"];
  onClose: () => void;
}

interface EditConnectionModalContentProps {
  isOpen: boolean;
  initialConnection: Connection;
  onClose: () => void;
}

const EditConnectionModalContent: FC<EditConnectionModalContentProps> = ({
  isOpen,
  initialConnection,
  onClose,
}) => {
  const state = useConnectionFormState();
  const { setPhase } = useConnectionFormActions();
  const { toast } = useToast();
  const [connection] = useState(initialConnection);

  const { mutate: updateConnection } = useUpdateConnectionMutation();

  const handleUpdateConnection = useCallback(
    (config: ConnectionFormState["config"]) => {
      const name = state.name.trim();
      if (!name) return;

      setPhase(ConnectionFormPhase.SUBMITTING);

      updateConnection(
        create(UpdateConnectionRequestSchema, {
          connection: create(ConnectionSchema, {
            id: connection.id,
            kind: connection.kind,
            connector: connection.connector,
            name,
            config,
            secretRefs: connection.secretRefs,
            version: connection.version,
          }),
        }),
        {
          onSuccess: () => {
            toast({
              variant: ToastVariant.SUCCESS,
              header: "Connection updated",
            });
            onClose();
          },
          onError: (error) => {
            setPhase(ConnectionFormPhase.ERROR);
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
    },
    [state.name, connection, updateConnection, toast, onClose, setPhase],
  );

  return (
    <ConnectionForm
      isOpen={isOpen}
      connectorName={connection.connector}
      connectorKind={connection.kind}
      connectionId={connection.id}
      secretRefs={connection.secretRefs}
      onSubmit={handleUpdateConnection}
      onClose={onClose}
    />
  );
};

const EditConnectionModal: FC<EditConnectionModalProps> = ({ isOpen, connectionId, onClose }) => {
  const session = useOverlaySession(isOpen);

  const { data, error, isError } = useGetConnectionQuery({
    input: createGetConnectionInput(connectionId ?? ""),
    options: { enabled: !!session && !!connectionId, retry: false },
  });
  const connection = data?.connection;

  if (!session) {
    return null;
  }

  if (isError) {
    const isNotFound = isNotFoundError(error);

    return (
      <ConnectionFormWrapper
        isOpen={isOpen}
        size={ModalSize.MEDIUM}
        header="Edit connection"
        onClose={onClose}
      >
        <ErrorLayout
          header={isNotFound ? "Connection not found" : "Failed to load connection"}
          description={
            isNotFound
              ? "This connection no longer exists."
              : "The connection could not be loaded. Try again later."
          }
          detail={IS_DEBUG ? error?.message : undefined}
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      </ConnectionFormWrapper>
    );
  }

  if (!connection) {
    return (
      <ConnectionFormWrapper
        isOpen={isOpen}
        size={ModalSize.MEDIUM}
        header="Edit connection"
        onClose={onClose}
      >
        <PendingLayout />
      </ConnectionFormWrapper>
    );
  }

  return (
    <ConnectionFormProvider
      key={session}
      initialState={{ name: connection.name, config: connection.config ?? {} }}
    >
      <EditConnectionModalContent
        isOpen={isOpen}
        initialConnection={connection}
        onClose={onClose}
      />
    </ConnectionFormProvider>
  );
};

export default EditConnectionModal;
