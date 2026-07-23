import { useState } from "react";

import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import ConnectionDrawerConfiguration from "@/pages/connectors/components/drawer/ConnectionDrawerConfiguration";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import ConnectionDrawerPipelines from "@/pages/connectors/components/drawer/ConnectionDrawerPipelines";
import ConnectionDrawerSecrets from "@/pages/connectors/components/drawer/ConnectionDrawerSecrets";

import { useDeleteConnectionMutation } from "@/api/queries/connectors";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";
import { InputSize } from "@galaxy-io/dls/inputs/Input";

const DrawerWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const DrawerBody = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  overflow-y: auto;
  background-color: ${({ theme }) => theme.color.background.base};
`);

interface ConnectionDrawerProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawer = ({ connection, onClose }: ConnectionDrawerProps) => {
  const navigate = useNavigate();
  const { mutate: deleteConnection, isPending: isDeleting } =
    useDeleteConnectionMutation();
  const { showToast } = useToast();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

  const handleOpenDeleteModal = () => setIsDeleteModalOpen(true);
  const handleCloseDeleteModal = () => setIsDeleteModalOpen(false);

  const handleConfirmDelete = () => {
    deleteConnection(
      { id: connection.id },
      {
        onSuccess: () => {
          showToast({
            header: "Connection deleted",
            subheader: `${connection.name} has been deleted successfully.`,
            variant: ToastVariant.SUCCESS,
          });
          onClose();
          navigate({
            to: connection.kind === ConnectorKind.SINK ? "/sinks" : "/sources",
          });
        },
        onError: (error) => {
          showToast({
            header: "Delete failed",
            subheader:
              error instanceof Error
                ? error.message
                : "Failed to delete connection",
            variant: ToastVariant.ERROR,
          });
          setIsDeleteModalOpen(false);
        },
      },
    );
  };

  return (
    <DrawerWrapper>
      <ConnectionDrawerHeader connection={connection} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          gap={12}
          padding="16px"
          fillWidth
        >
          <ConnectionDrawerList>
            <ConnectionDrawerKeyValueRow
              label="Connection ID"
              value={
                <CopyInput
                  value={connection.id}
                  size={InputSize.SMALL}
                  width={272}
                  isMonospace
                />
                // <Text size={TextSize.BODY_SM} isMonospace isSelectable>
                //   {connection.id}
                // </Text>
              }
            />
            <ConnectionDrawerKeyValueRow
              label="Connector"
              value={
                <Text size={TextSize.BODY_SM}>{connection.connector}</Text>
              }
            />
            <ConnectionDrawerKeyValueRow
              label="Kind"
              value={
                <Chip
                  label={
                    connection.kind === ConnectorKind.SOURCE ? "Source" : "Sink"
                  }
                  variant={
                    connection.kind === ConnectorKind.SOURCE
                      ? ChipVariant.LIME
                      : ChipVariant.PINK
                  }
                />
              }
            />
            <ConnectionDrawerKeyValueRow
              label="Version"
              value={
                <Text size={TextSize.BODY_SM}>
                  {connection.version.toString()}
                </Text>
              }
            />
          </ConnectionDrawerList>

          <ConnectionDrawerConfiguration config={connection.config} />

          <ConnectionDrawerSecrets secretRefs={connection.secretRefs} />

          <ConnectionDrawerPipelines connectionId={connection.id} />
        </FlexWrapper>
      </DrawerBody>

      <HorizontalDivider />

      <FlexItem shrink={0} grow={0} padding="16px" fillWidth>
        <DangerZone
          title="Delete connection"
          description="This will permanently delete this connection."
          buttonLabel="Delete"
          onAction={handleOpenDeleteModal}
        />
      </FlexItem>

      <Modal open={isDeleteModalOpen} onClose={handleCloseDeleteModal}>
        <DeleteConfirmDialog
          open={isDeleteModalOpen}
          onClose={handleCloseDeleteModal}
          onConfirm={handleConfirmDelete}
          title="Delete connection"
          body="This will permanently delete this connection and all associated data."
          confirmationPhrase={connection.name || ""}
          confirmLabel="Delete connection"
          isPending={isDeleting}
        />
      </Modal>
    </DrawerWrapper>
  );
};

export default ConnectionDrawer;
