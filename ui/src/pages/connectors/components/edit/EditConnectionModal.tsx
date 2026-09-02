import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type Connection,
  ConnectionSchema,
  GetConnectionRequestSchema,
  UpdateConnectionRequestSchema,
} from "@/gen/ingestion/v1/connections_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import { ConnectionFormActionType } from "@/pages/connectors/components/form/actions";
import ConnectionForm from "@/pages/connectors/components/form/ConnectionForm";
import ConnectionFormProvider, {
  useConnectionFormContext,
} from "@/pages/connectors/components/form/ConnectionFormProvider";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { ConnectionFormPhase } from "@/pages/connectors/components/form/types";
import { CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH } from "@/pages/connectors/constants";

import { useGetConnectionQuery, useUpdateConnectionMutation } from "@/api/queries/connections";

import { getErrorMessage } from "@/utils/errors";

interface EditConnectionModalProps {
  onClose: () => void;
}

interface EditConnectionModalContentProps extends EditConnectionModalProps {
  connection: Connection;
}

const EditConnectionModalContent = ({ connection, onClose }: EditConnectionModalContentProps) => {
  const { state, dispatch } = useConnectionFormContext();
  const { showToast } = useToast();

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
          tenantId: connection.tenantId,
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
  }, [state.name, state.config, connection, updateConnection, showToast, onClose, dispatch]);

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

const EditConnectionModal = ({ onClose }: EditConnectionModalProps) => {
  const { connectionId } = useSearch({ from: "/_app" });

  const { data, isError } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId ?? "" }),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = data?.connection;

  if (isError) {
    return (
      <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
        <ErrorLayout
          icon={<Icon component={WarningCircleIcon} size={24} variant={IconVariant.ERROR} />}
          header="Connection not found"
          message="This connection no longer exists."
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      </ConnectionFormWrapper>
    );
  }

  if (!connection) {
    return (
      <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH}>
        <FlexWrapper alignItems={AlignItems.CENTER} padding="12px 16px" gap={12} fillWidth>
          <TextShimmer height={36} width={36} />
          <TextShimmer height={20} width={220} />
        </FlexWrapper>
        <HorizontalDivider />
        <FlexWrapper direction={FlexDirection.COLUMN} padding="16px" gap={16} fillWidth>
          <TextShimmer height={32} width="100%" />
          <TextShimmer height={32} width="100%" />
          <TextShimmer height={32} width="100%" />
          <TextShimmer height={32} width="100%" />
        </FlexWrapper>
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
